CREATE TABLE plop.payloads (
    id          TEXT        PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    text        TEXT,
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    files       JSONB       NOT NULL DEFAULT '[]',
    acked       BOOLEAN     NOT NULL DEFAULT false
);

CREATE INDEX idx_payloads_expires_at ON payloads (expires_at)
    WHERE acked = false;