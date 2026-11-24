CREATE TABLE sites (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),public_id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
 user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,name text NOT NULL,hostname text NOT NULL,
 timezone text NOT NULL,status text NOT NULL CHECK(status IN ('active','disabled')),created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(user_id,hostname)
);
CREATE TABLE site_origins(site_id uuid REFERENCES sites ON DELETE CASCADE,origin text NOT NULL,PRIMARY KEY(site_id,origin));
CREATE TABLE web_events(
 site_id uuid REFERENCES sites ON DELETE CASCADE,id uuid NOT NULL,PRIMARY KEY(site_id,id),occurred_at timestamptz NOT NULL,received_at timestamptz NOT NULL,
 kind text NOT NULL CHECK(kind IN ('pageview','custom')),name text NOT NULL,path text NOT NULL,referrer text NOT NULL,
 source text NOT NULL,medium text NOT NULL,campaign text NOT NULL,properties jsonb NOT NULL,device text NOT NULL,browser text NOT NULL,os text NOT NULL,
 visitor_hash bytea NOT NULL CHECK(octet_length(visitor_hash)=32)
);
CREATE INDEX events_site_time ON web_events(site_id,received_at DESC);
CREATE INDEX events_retention ON web_events(received_at);
