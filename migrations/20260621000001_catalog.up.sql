CREATE TABLE product (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    api_key    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plan (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES product(id),
    code       TEXT NOT NULL,
    name       TEXT NOT NULL,
    limits     JSONB NOT NULL DEFAULT '{}'::jsonb,
    trial_days INT NOT NULL DEFAULT 14,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, code)
);

CREATE TABLE plan_price (
    id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_id  BIGINT NOT NULL REFERENCES plan(id),
    currency TEXT NOT NULL,
    interval TEXT NOT NULL CHECK (interval IN ('month','year')),
    amount   BIGINT NOT NULL CHECK (amount >= 0),
    UNIQUE (plan_id, currency, interval)
);

CREATE TABLE webhook_endpoint (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES product(id),
    url        TEXT NOT NULL,
    secret     TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
