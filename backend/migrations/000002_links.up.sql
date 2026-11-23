CREATE TABLE domains (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid REFERENCES users ON DELETE CASCADE,
 hostname text NOT NULL UNIQUE CHECK(hostname=lower(hostname)), kind text NOT NULL CHECK(kind IN ('system','subdomain','custom')),
 status text NOT NULL CHECK(status IN ('pending','verified','active','error')), challenge text,
 challenge_expires_at timestamptz, verified_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 CHECK ((kind='system')=(user_id IS NULL))
);
CREATE UNIQUE INDEX one_personal ON domains(user_id) WHERE kind='subdomain';
INSERT INTO domains(hostname,kind,status) VALUES ('go.saturnita.ru','system','active');
CREATE TABLE links (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
 domain_id uuid NOT NULL REFERENCES domains ON DELETE RESTRICT, name text NOT NULL, slug text NOT NULL,
 target_url text NOT NULL, target_host text NOT NULL, utm jsonb NOT NULL DEFAULT '{}',
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','inactive','archived')),
 expires_at timestamptz, click_limit bigint CHECK(click_limit>0), click_count bigint NOT NULL DEFAULT 0 CHECK(click_count>=0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(domain_id,slug), CHECK(click_limit IS NULL OR click_limit>=click_count)
);
CREATE INDEX links_owner ON links(user_id,status,created_at DESC);
CREATE INDEX links_target_host ON links(target_host);
