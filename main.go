package main

import (
	"context"
	"net/http"
	"time"

	"billing/base"
	"billing/catalog"
	"billing/config"
	"billing/customers"
	"billing/jobs"
	"billing/middleware"
	"billing/payments"
	"billing/payments/kaspi"
	"billing/payments/payme"
	"billing/subscriptions"
	"billing/webhooks"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg := config.Load()

	// Root context for boot + background workers; cancelled on shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Retry migrations: postgres may still be starting (boot-race). Wait and
	// retry transient connect/DNS failures instead of crash-looping; only fatal
	// once the budget is exhausted.
	if err := retryDBConnect(ctx, "migrate", migratePolicy, func(context.Context) error {
		return Migrate(cfg.DatabaseURL)
	}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	pool, err := newPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// Payment provider selection: Payme (recurrent) when configured, else manual.
	var provider payments.Provider = payments.NewManual()
	var methods *payments.PaymentMethodRepo
	var cardBinder payments.CardBinder
	if cfg.PaymeURL != "" {
		cipher, err := base.NewCipher(cfg.TokenEncKey)
		if err != nil {
			log.Fatalf("PAYME_TOKEN_ENC_KEY: %v", err)
		}
		methods = payments.NewPaymentMethodRepo(cipher)
		pc := payme.NewClient(cfg.PaymeURL, cfg.PaymeMerch, cfg.PaymeKey, nil)
		provider = payme.NewProvider(pc)
		cardBinder = pc
	}
	paySvc := payments.NewService(provider, methods)

	// gin.New (not Default) so request logging is our structured logger, not
	// gin's text access log; Recovery still guards against handler panics.
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestLogger())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// authenticated client API
	v1 := r.Group("/v1")
	v1.Use(middleware.APIKey(apiKeyResolver(pool)))
	catalog.GetRoutes(v1, catalog.NewController(catalog.NewService(pool)))
	customers.GetRoutes(v1, customers.NewController(pool))
	subscriptions.GetRoutes(v1, subscriptions.NewController(pool))
	if cardBinder != nil {
		payments.GetCardRoutes(v1, payments.NewCardController(pool, cardBinder, methods))
	}

	// Kaspi QR (poll-based) — active when configured.
	var kaspiPoller *kaspi.Poller
	if cfg.KaspiURL != "" {
		kc := kaspi.NewClient(cfg.KaspiURL, cfg.KaspiAPIKey, cfg.KaspiDevice, cfg.KaspiOrgBIN, nil)
		krepo := kaspi.NewRepo()
		kaspiPoller = kaspi.NewPoller(kc, krepo, paySvc)
		kaspi.GetRoutes(v1, kaspi.NewController(pool, kc, krepo))
	}

	// admin API (same api-key guard for SP1; tighten later)
	admin := v1.Group("/admin")
	payments.GetAdminRoutes(admin, payments.NewController(pool))
	catalog.GetAdminRoutes(admin, catalog.NewAdminController(pool))

	// background workers
	go runWorkers(ctx, pool, cfg.GraceDays, paySvc, kaspiPoller)

	log.Infof("billing listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}

// newPool builds a tuned pgxpool. pgxpool creates connections lazily and
// re-resolves DNS on every new dial, so a postgres recreated with a new IP is
// picked up automatically. The lifetime/health-check settings ensure conns that
// died when postgres restarted are rotated out and replaced with fresh ones.
func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 15 * time.Second // evict dead conns shortly after a restart
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

// logConnErr logs op's failure, demoting transient connect/DNS errors to warn so
// a postgres restart or IP change does not spam error logs, while keeping real
// config rejections and logic errors at error.
func logConnErr(op string, err error) {
	switch {
	case base.IsConfigConnErr(err):
		log.Errorf("%s: %v", op, err)
	case base.IsRetryableConnErr(err):
		log.Warnf("%s: transient, retrying next tick: %v", op, err)
	default:
		log.Errorf("%s: %v", op, err)
	}
}

// runWorkers ticks the lifecycle + dispatcher jobs every 30s. The loop only
// exits on ctx cancellation: transient DB errors are logged and skipped, so the
// next tick re-resolves DNS and acquires a fresh connection from the pool. A
// postgres restart therefore self-heals within a tick without restarting billing.
func runWorkers(ctx context.Context, pool *pgxpool.Pool, graceDays int, charger jobs.Charger, kaspiPoller *kaspi.Poller) {
	runner := jobs.NewRunner(graceDays, charger)
	disp := webhooks.NewDispatcher(nil)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runJob(ctx, pool, runner, disp, kaspiPoller)
		}
	}
}

// runJob opens one tx per tick and runs all workers within it.
func runJob(ctx context.Context, pool *pgxpool.Pool, runner *jobs.Runner, disp *webhooks.Dispatcher, kaspiPoller *kaspi.Poller) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		logConnErr("worker begin", err)
		return
	}
	defer tx.Rollback(ctx)
	if err := runner.TrialExpiry(ctx, tx); err != nil {
		logConnErr("trial expiry", err)
		return
	}
	if err := runner.GraceExpiry(ctx, tx); err != nil {
		logConnErr("grace expiry", err)
		return
	}
	if err := runner.RenewalDue(ctx, tx); err != nil {
		logConnErr("renewal", err)
		return
	}
	if err := runner.DunningCharge(ctx, tx); err != nil {
		logConnErr("dunning", err)
		return
	}
	if kaspiPoller != nil {
		if err := kaspiPoller.Poll(ctx, tx); err != nil {
			logConnErr("kaspi poll", err)
			return
		}
	}
	if _, err := disp.DeliverBatch(ctx, tx, 50); err != nil {
		logConnErr("dispatch", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		logConnErr("worker commit", err)
	}
}
