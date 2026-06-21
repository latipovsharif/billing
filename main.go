package main

import (
	"context"
	"net/http"
	"time"

	"billing/catalog"
	"billing/config"
	"billing/customers"
	"billing/jobs"
	"billing/middleware"
	"billing/payments"
	"billing/subscriptions"
	"billing/webhooks"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg := config.Load()

	if err := Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// authenticated client API
	v1 := r.Group("/v1")
	v1.Use(middleware.APIKey(apiKeyResolver(pool)))
	catalog.GetRoutes(v1, catalog.NewController(catalog.NewService(pool)))
	customers.GetRoutes(v1, customers.NewController(pool))
	subscriptions.GetRoutes(v1, subscriptions.NewController(pool))

	// admin API (same api-key guard for SP1; tighten later)
	admin := v1.Group("/admin")
	payments.GetAdminRoutes(admin, payments.NewController(pool))

	// background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWorkers(ctx, pool, cfg.GraceDays)

	log.Infof("billing listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}

// runWorkers ticks the lifecycle + dispatcher jobs every 30s.
func runWorkers(ctx context.Context, pool *pgxpool.Pool, graceDays int) {
	runner := jobs.NewRunner(graceDays)
	disp := webhooks.NewDispatcher(nil)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runJob(ctx, pool, runner, disp)
		}
	}
}

// runJob opens one tx per tick and runs all workers within it.
func runJob(ctx context.Context, pool *pgxpool.Pool, runner *jobs.Runner, disp *webhooks.Dispatcher) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Errorf("worker begin: %v", err)
		return
	}
	defer tx.Rollback(ctx)
	if err := runner.TrialExpiry(ctx, tx); err != nil {
		log.Errorf("trial expiry: %v", err)
		return
	}
	if err := runner.GraceExpiry(ctx, tx); err != nil {
		log.Errorf("grace expiry: %v", err)
		return
	}
	if err := runner.RenewalDue(ctx, tx); err != nil {
		log.Errorf("renewal: %v", err)
		return
	}
	if _, err := disp.DeliverBatch(ctx, tx, 50); err != nil {
		log.Errorf("dispatch: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Errorf("worker commit: %v", err)
	}
}
