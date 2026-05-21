CREATE TABLE plop.users (
    username        TEXT        PRIMARY KEY,
    password_hash   TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);