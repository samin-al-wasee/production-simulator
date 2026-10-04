-- Identity: users, their OAuth accounts, and database-backed sessions
-- (ADR-0031). No passwords are stored; a user is keyed by its provider
-- identity and signs in through OAuth.

CREATE TABLE users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email        text NOT NULL DEFAULT '',
    display_name text NOT NULL DEFAULT '',
    avatar_url   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- One user per email, but only when a provider gave us one.
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email <> '';

CREATE TABLE oauth_accounts (
    provider         text NOT NULL,
    provider_user_id text NOT NULL,
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, provider_user_id)
);

CREATE INDEX oauth_accounts_user_id_idx ON oauth_accounts (user_id);

CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
