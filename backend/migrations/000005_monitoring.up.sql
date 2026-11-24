ALTER TABLE links ADD UNIQUE(id,user_id);
CREATE TABLE monitors (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
 link_id uuid UNIQUE, FOREIGN KEY(link_id,user_id) REFERENCES links(id,user_id) ON DELETE CASCADE,
 name varchar(160) NOT NULL,url varchar(2048) NOT NULL,kind text NOT NULL CHECK(kind IN ('website','api','link_target')),
 CHECK((kind='link_target')=(link_id IS NOT NULL)),method text NOT NULL CHECK(method IN ('HEAD','GET')),
 interval_seconds integer NOT NULL CHECK(interval_seconds BETWEEN 300 AND 86400),timeout_ms integer NOT NULL CHECK(timeout_ms BETWEEN 500 AND 30000),
 status_min integer NOT NULL,status_max integer NOT NULL,CHECK(status_min BETWEEN 100 AND status_max AND status_max<=599),
 enabled boolean NOT NULL DEFAULT true,status text NOT NULL DEFAULT 'unknown' CHECK(status IN ('unknown','operational','degraded','down','paused')),
 failures integer NOT NULL DEFAULT 0,next_check_at timestamptz NOT NULL DEFAULT now(),lease uuid,lease_until timestamptz,
 last_checked_at timestamptz,last_success_at timestamptz,manual_at timestamptz,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX monitors_due ON monitors(next_check_at) WHERE enabled;
CREATE INDEX monitors_owner ON monitors(user_id,created_at DESC);
CREATE TABLE monitor_checks (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), monitor_id uuid NOT NULL REFERENCES monitors ON DELETE CASCADE,
 scheduled_at timestamptz NOT NULL, checked_at timestamptz NOT NULL DEFAULT now(),http_status integer,latency_ms integer NOT NULL CHECK(latency_ms>=0),success boolean NOT NULL,error_type text,
 UNIQUE(monitor_id,scheduled_at)
);
CREATE INDEX checks_history ON monitor_checks(monitor_id,checked_at DESC);
CREATE INDEX checks_retention ON monitor_checks(checked_at);
CREATE TABLE incidents (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), monitor_id uuid NOT NULL REFERENCES monitors ON DELETE CASCADE,
 started_at timestamptz NOT NULL DEFAULT now(),ended_at timestamptz,cause text NOT NULL
);
CREATE UNIQUE INDEX incident_open ON incidents(monitor_id) WHERE ended_at IS NULL;
CREATE INDEX incident_history ON incidents(monitor_id,started_at DESC);
