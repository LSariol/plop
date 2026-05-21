CREATE TABLE plop.sessions (
    token       TEXT        PRIMARY KEY,
    username    TEXT        NOT NULL REFERENCES plop.users(username) ON DELETE CASCADE, 
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);