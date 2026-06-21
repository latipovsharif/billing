CREATE TABLE payment_method (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customer(id),
    provider    TEXT NOT NULL,
    token_enc   BYTEA NOT NULL,
    card_masked TEXT NOT NULL,
    expire      TEXT NOT NULL,
    verified    BOOLEAN NOT NULL DEFAULT FALSE,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_default_payment_method
    ON payment_method (customer_id) WHERE is_default;
