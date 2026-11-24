CREATE TABLE users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email varchar(320) NOT NULL UNIQUE CHECK(email=lower(email)),
 display_name varchar(120) NOT NULL,password_hash text NOT NULL,
 timezone text NOT NULL DEFAULT 'UTC',locale text NOT NULL DEFAULT 'ru',preferences jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
 token_hash bytea NOT NULL UNIQUE,user_agent varchar(512) NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),last_seen_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '14 days',revoked_at timestamptz
);
CREATE INDEX sessions_owner ON sessions(user_id,created_at DESC);
