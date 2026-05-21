CREATE TABLE plop.desktops (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES plop.users(username) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    token      TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plop.pairing_codes (
    code       TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES plop.users(username) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE plop.payloads ADD COLUMN user_id TEXT REFERENCES plop.users(username);
CREATE INDEX idx_payloads_user_id ON payloads (user_id) WHERE acked = false;
