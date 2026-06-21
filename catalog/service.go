package catalog

import (
	"context"

	"billing/base"
)

// Service exposes catalog reads over a pool.
type Service struct {
	db   base.PGXDB
	repo *Repo
}

// NewService builds a catalog service from anything that can query (a pool).
func NewService(pool base.PGXDB) *Service {
	return &Service{db: pool, repo: NewRepo()}
}

func (s *Service) ListPlans(ctx context.Context, productID int64) ([]PlanWithPrices, error) {
	return s.repo.ListPlansWithPrices(ctx, s.db, productID)
}
