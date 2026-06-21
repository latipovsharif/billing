package customers

import (
	"context"

	"billing/base"
)

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

// Register upserts a customer by (product_id, external_ref) — idempotent.
func (r *Repo) Register(ctx context.Context, db base.PGXDB, productID int64, externalRef, ownerUserID, displayName, currency string) (Customer, error) {
	var c Customer
	err := db.QueryRow(ctx,
		`INSERT INTO customer (product_id, external_ref, owner_user_id, display_name, default_currency)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (product_id, external_ref)
		 DO UPDATE SET display_name=EXCLUDED.display_name, owner_user_id=EXCLUDED.owner_user_id
		 RETURNING id, product_id, external_ref, owner_user_id, display_name, default_currency`,
		productID, externalRef, ownerUserID, displayName, currency).
		Scan(&c.ID, &c.ProductID, &c.ExternalRef, &c.OwnerUserID, &c.DisplayName, &c.DefaultCurrency)
	return c, err
}

// ByRef loads a customer by product + external ref.
func (r *Repo) ByRef(ctx context.Context, db base.PGXDB, productID int64, externalRef string) (Customer, bool, error) {
	var c Customer
	err := db.QueryRow(ctx,
		`SELECT id, product_id, external_ref, owner_user_id, display_name, default_currency
		 FROM customer WHERE product_id=$1 AND external_ref=$2`, productID, externalRef).
		Scan(&c.ID, &c.ProductID, &c.ExternalRef, &c.OwnerUserID, &c.DisplayName, &c.DefaultCurrency)
	if base.IsNotFound(err) {
		return Customer{}, false, nil
	}
	return c, err == nil, err
}
