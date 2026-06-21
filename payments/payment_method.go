package payments

import (
	"context"

	"billing/base"
)

// PaymentMethodRepo stores card payment methods with encrypted tokens.
type PaymentMethodRepo struct{ cipher *base.Cipher }

func NewPaymentMethodRepo(cipher *base.Cipher) *PaymentMethodRepo {
	return &PaymentMethodRepo{cipher: cipher}
}

// SaveCard stores an unverified card (encrypted token + masked PAN). Returns id.
func (r *PaymentMethodRepo) SaveCard(ctx context.Context, db base.PGXDB, customerID int64, provider, token, masked, expire string) (int64, error) {
	enc, err := r.cipher.Encrypt([]byte(token))
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(ctx,
		`INSERT INTO payment_method (customer_id, provider, token_enc, card_masked, expire)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		customerID, provider, enc, masked, expire).Scan(&id)
	return id, err
}

// MarkVerifiedDefault marks the method verified and the customer's default,
// clearing any previous default first.
func (r *PaymentMethodRepo) MarkVerifiedDefault(ctx context.Context, db base.PGXDB, id, customerID int64) error {
	if _, err := db.Exec(ctx,
		`UPDATE payment_method SET is_default=FALSE WHERE customer_id=$1 AND is_default`, customerID); err != nil {
		return err
	}
	_, err := db.Exec(ctx,
		`UPDATE payment_method SET verified=TRUE, is_default=TRUE WHERE id=$1 AND customer_id=$2`, id, customerID)
	return err
}

// DefaultVerifiedToken returns the decrypted token of the customer's default
// verified method, or ok=false if none.
func (r *PaymentMethodRepo) DefaultVerifiedToken(ctx context.Context, db base.PGXDB, customerID int64) (string, bool, error) {
	var enc []byte
	err := db.QueryRow(ctx,
		`SELECT token_enc FROM payment_method WHERE customer_id=$1 AND verified AND is_default LIMIT 1`,
		customerID).Scan(&enc)
	if base.IsNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	tok, err := r.cipher.Decrypt(enc)
	if err != nil {
		return "", false, err
	}
	return string(tok), true, nil
}

// Card is the safe (no token) view for listing.
type Card struct {
	ID       int64  `json:"id"`
	Masked   string `json:"card_masked"`
	Expire   string `json:"expire"`
	Verified bool   `json:"verified"`
	Default  bool   `json:"is_default"`
}

// List returns the customer's cards without tokens.
func (r *PaymentMethodRepo) List(ctx context.Context, db base.PGXDB, customerID int64) ([]Card, error) {
	rows, err := db.Query(ctx,
		`SELECT id, card_masked, expire, verified, is_default FROM payment_method WHERE customer_id=$1 ORDER BY id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.Masked, &c.Expire, &c.Verified, &c.Default); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TokenByID returns the decrypted token + customer for one method.
func (r *PaymentMethodRepo) TokenByID(ctx context.Context, db base.PGXDB, id int64) (token string, customerID int64, ok bool, err error) {
	var enc []byte
	err = db.QueryRow(ctx, `SELECT token_enc, customer_id FROM payment_method WHERE id=$1`, id).Scan(&enc, &customerID)
	if base.IsNotFound(err) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	tok, err := r.cipher.Decrypt(enc)
	if err != nil {
		return "", 0, false, err
	}
	return string(tok), customerID, true, nil
}

// Delete removes a method row.
func (r *PaymentMethodRepo) Delete(ctx context.Context, db base.PGXDB, id int64) error {
	_, err := db.Exec(ctx, `DELETE FROM payment_method WHERE id=$1`, id)
	return err
}
