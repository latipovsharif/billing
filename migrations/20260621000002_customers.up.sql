CREATE TABLE customer (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id       BIGINT NOT NULL REFERENCES product(id),
    external_ref     TEXT NOT NULL,
    owner_user_id    TEXT NOT NULL,
    display_name     TEXT NOT NULL,
    default_currency TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, external_ref)
);
