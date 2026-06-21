package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// apiKeyResolver returns a resolver backed by the product table.
func apiKeyResolver(pool *pgxpool.Pool) func(string) (int64, bool) {
	return func(key string) (int64, bool) {
		var id int64
		err := pool.QueryRow(context.Background(),
			`SELECT id FROM product WHERE api_key=$1 AND active`, key).Scan(&id)
		if err != nil {
			return 0, false
		}
		return id, true
	}
}
