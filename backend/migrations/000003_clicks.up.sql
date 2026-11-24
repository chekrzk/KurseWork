CREATE TABLE link_clicks (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),link_id uuid NOT NULL REFERENCES links ON DELETE CASCADE,
 occurred_at timestamptz NOT NULL,referrer text NOT NULL,source text NOT NULL,medium text NOT NULL,campaign text NOT NULL,
 device text NOT NULL,browser text NOT NULL,os text NOT NULL,is_bot boolean NOT NULL, visitor_hash bytea NOT NULL CHECK(octet_length(visitor_hash)=32)
);
CREATE INDEX clicks_link_time ON link_clicks(link_id,occurred_at DESC);
CREATE INDEX clicks_retention ON link_clicks(occurred_at);
-- Local installations share the generated fallback key across restarts and processes.
CREATE TABLE local_installation_key (singleton boolean PRIMARY KEY CHECK(singleton),secret bytea NOT NULL);
INSERT INTO local_installation_key VALUES(true,uuid_send(gen_random_uuid())||uuid_send(gen_random_uuid()));
