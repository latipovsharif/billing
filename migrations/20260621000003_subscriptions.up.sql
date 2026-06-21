CREATE TABLE subscription (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id          BIGINT NOT NULL REFERENCES customer(id),
    plan_id              BIGINT NOT NULL REFERENCES plan(id),
    status               TEXT NOT NULL,
    currency             TEXT NOT NULL,
    interval             TEXT NOT NULL CHECK (interval IN ('month','year')),
    amount               BIGINT NOT NULL CHECK (amount >= 0),
    trial_end            TIMESTAMPTZ,
    current_period_start TIMESTAMPTZ,
    current_period_end   TIMESTAMPTZ,
    cancel_at            TIMESTAMPTZ,
    canceled_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- At most one live subscription per customer.
CREATE UNIQUE INDEX one_live_subscription
    ON subscription (customer_id)
    WHERE status IN ('trialing','active','past_due','suspended');

CREATE TABLE subscription_status_history (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES subscription(id),
    from_status     TEXT,
    to_status       TEXT NOT NULL,
    reason          TEXT NOT NULL,
    actor           TEXT NOT NULL CHECK (actor IN ('system','operator','provider')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoice (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES subscription(id),
    customer_id     BIGINT NOT NULL REFERENCES customer(id),
    currency        TEXT NOT NULL,
    amount          BIGINT NOT NULL CHECK (amount >= 0),
    status          TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','paid','void')),
    period_start    TIMESTAMPTZ NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    due_date        TIMESTAMPTZ NOT NULL,
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at         TIMESTAMPTZ,
    UNIQUE (subscription_id, period_start)
);

CREATE TABLE payment (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id   BIGINT NOT NULL REFERENCES invoice(id),
    provider     TEXT NOT NULL,
    provider_ref TEXT,
    currency     TEXT NOT NULL,
    amount       BIGINT NOT NULL CHECK (amount >= 0),
    status       TEXT NOT NULL,
    paid_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw          JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE ledger_entry (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customer(id),
    invoice_id  BIGINT REFERENCES invoice(id),
    payment_id  BIGINT REFERENCES payment(id),
    type        TEXT NOT NULL CHECK (type IN ('charge','payment','refund','adjustment')),
    currency    TEXT NOT NULL,
    amount      BIGINT NOT NULL,
    ref         TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_outbox (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      BIGINT NOT NULL REFERENCES product(id),
    event_id        TEXT NOT NULL UNIQUE,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','delivered','failed')),
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX webhook_outbox_due ON webhook_outbox (next_attempt_at) WHERE status='pending';
