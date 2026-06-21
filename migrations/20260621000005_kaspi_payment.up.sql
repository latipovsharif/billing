CREATE TABLE kaspi_payment (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id    BIGINT NOT NULL REFERENCES invoice(id),
    qr_invoice_id TEXT NOT NULL,
    qr_token      TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','canceled','expired')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_pending_kaspi_payment ON kaspi_payment (invoice_id) WHERE status='pending';
