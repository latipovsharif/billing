package customers

// Customer is a tenant of a product (e.g. one shop in cloudmarket).
type Customer struct {
	ID              int64  `json:"id"`
	ProductID       int64  `json:"product_id"`
	ExternalRef     string `json:"external_ref"`
	OwnerUserID     string `json:"owner_user_id"`
	DisplayName     string `json:"display_name"`
	DefaultCurrency string `json:"default_currency"`
}
