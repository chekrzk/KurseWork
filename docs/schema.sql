-- Saturnita Pulse | проект PostgreSQL-схемы этапа 1

create table users (
  id uuid primary key,
  email varchar(320) not null,
  password_hash text not null,
  display_name varchar(120) not null,
  timezone varchar(64) not null default 'UTC',
  locale varchar(10) not null default 'ru',
  preferences jsonb not null default '{}'::jsonb,
  created_at timestamptz not null,
  updated_at timestamptz not null
);

create table sessions (
  id uuid primary key,
  user_id uuid not null references users(id) on delete CASCADE,
  token_hash bytea not null unique,
  csrf_secret_hash bytea,
  user_agent varchar(512),
  created_at timestamptz not null,
  last_seen_at timestamptz not null,
  expires_at timestamptz not null,
  revoked_at timestamptz
);

create table domains (
  id uuid primary key,
  user_id uuid references users(id) on delete CASCADE,
  kind text not null check (kind in ('system', 'subdomain', 'custom')),
  hostname varchar(253) not null,
  status text not null check (status in ('pending', 'verified', 'active', 'error', 'disabled')),
  verification_error varchar(500),
  verified_at timestamptz,
  activated_at timestamptz,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  check ((kind = 'system') = (user_id is null)),
  check (hostname = lower(hostname))
);

create table domain_verifications (
  id uuid primary key,
  domain_id uuid not null references domains(id) on delete CASCADE,
  token varchar(128) not null,
  txt_name varchar(253) not null,
  created_at timestamptz not null,
  expires_at timestamptz not null,
  checked_at timestamptz,
  succeeded_at timestamptz
);

create table links (
  id uuid primary key,
  user_id uuid not null references users(id) on delete CASCADE,
  domain_id uuid not null references domains(id) on delete RESTRICT,
  name varchar(160) not null,
  slug varchar(100) not null,
  target_url varchar(2048) not null,
  utm_source varchar(255),
  utm_medium varchar(255),
  utm_campaign varchar(255),
  utm_term varchar(255),
  utm_content varchar(255),
  status text not null check (status in ('active', 'inactive', 'archived')),
  expires_at timestamptz,
  click_limit bigint,
  click_count bigint not null default 0,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique(domain_id,slug),
  unique(id,user_id),
  check(slug=lower(slug)),
  check(click_limit > 0 and click_count >= 0 and click_count <= click_limit)
);

create table link_clicks (
  id uuid primary key,
  link_id uuid not null references links(id) on delete CASCADE,
  user_id uuid not null references users(id) on delete CASCADE,
  clicked_at timestamptz not null,
  referer_host varchar(253),
  utm_source varchar(255),
  utm_medium varchar(255),
  utm_campaign varchar(255),
  device varchar(64),
  browser varchar(64),
  os varchar(64),
  country_code char(2),
  is_bot boolean not null default false,
  visitor_day_hash bytea,
  foreign key(link_id,user_id) references links(id,user_id) on delete cascade,
  check(octet_length(visitor_day_hash)=32)
);

create table sites (
  id uuid primary key,
  user_id uuid not null references users(id) on delete CASCADE,
  public_id uuid not null unique,
  name varchar(160) not null,
  hostname varchar(253) not null,
  timezone varchar(64) not null default 'UTC',
  status text not null check (status in ('active', 'disabled')),
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique(id,user_id)
);

create table site_origins (
  site_id uuid not null references sites(id) on delete CASCADE,
  origin varchar(512) not null,
  created_at timestamptz not null,
  primary key(site_id,origin)
);

create table web_events (
  id uuid primary key,
  site_id uuid not null references sites(id) on delete CASCADE,
  user_id uuid not null references users(id) on delete CASCADE,
  occurred_at timestamptz not null,
  received_at timestamptz not null,
  event_type text not null check (event_type in ('pageview', 'custom')),
  event_name varchar(80) not null,
  path varchar(2048),
  referrer_host varchar(253),
  utm_source varchar(255),
  utm_medium varchar(255),
  utm_campaign varchar(255),
  device varchar(64),
  browser varchar(64),
  os varchar(64),
  country_code char(2),
  visitor_day_hash bytea,
  properties jsonb not null default '{}',
  foreign key(site_id,user_id) references sites(id,user_id) on delete cascade,
  check(octet_length(visitor_day_hash)=32),
  check(jsonb_typeof(properties)='object')
);

create table monitors (
  id uuid primary key,
  user_id uuid not null references users(id) on delete CASCADE,
  link_id uuid references links(id) on delete CASCADE,
  name varchar(160) not null,
  url varchar(2048) not null,
  kind text not null check (kind in ('link_target', 'website', 'api')),
  method text not null check (method in ('GET', 'HEAD')),
  interval_seconds integer not null,
  timeout_ms integer not null,
  expected_status_min smallint not null,
  expected_status_max smallint not null,
  enabled boolean not null,
  status text not null check (status in ('operational', 'degraded', 'down', 'paused', 'unknown')),
  consecutive_failures integer not null default 0,
  last_checked_at timestamptz,
  last_success_at timestamptz,
  next_check_at timestamptz,
  lease_until timestamptz,
  lease_owner uuid,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique(id,user_id),
  foreign key(link_id,user_id) references links(id,user_id) on delete cascade,
  check ((kind='link_target') = (link_id is not null)),
  check(interval_seconds between 300 and 86400 and timeout_ms between 500 and 30000),
  check(expected_status_min between 100 and expected_status_max and expected_status_max <= 599)
);

create table monitor_checks (
  id uuid primary key,
  monitor_id uuid not null references monitors(id) on delete CASCADE,
  user_id uuid not null references users(id) on delete CASCADE,
  checked_at timestamptz not null,
  http_status smallint check (http_status between 100 and 599),
  latency_ms integer check (latency_ms >= 0),
  success boolean not null,
  error_type text,
  error_message varchar(500),
  resolved_ip inet,
  scheduled_at timestamptz not null,
  unique(monitor_id,scheduled_at),
  foreign key(monitor_id,user_id) references monitors(id,user_id) on delete cascade
);

create table incidents (
  id uuid primary key,
  monitor_id uuid not null references monitors(id) on delete CASCADE,
  user_id uuid not null references users(id) on delete CASCADE,
  started_at timestamptz not null,
  ended_at timestamptz,
  status text not null check (status in ('open', 'resolved')),
  cause text,
  first_check_id uuid references monitor_checks(id) on delete SET NULL,
  last_check_id uuid references monitor_checks(id) on delete SET NULL,
  created_at timestamptz not null,
  updated_at timestamptz not null,
  foreign key(monitor_id,user_id) references monitors(id,user_id) on delete cascade
);

create unique index users_email on users(lower(email));
create unique index domain_host on domains(lower(hostname));
create unique index personal_domain on domains(user_id) where kind='subdomain';
create unique index pending_challenge on domain_verifications(domain_id) where succeeded_at is null;
create unique index site_hostname on sites(user_id,lower(hostname));
create unique index link_monitor on monitors(link_id) where link_id is not null;
create unique index open_incident on incidents(monitor_id) where status='open';
create index session_owner on sessions(user_id,created_at desc);
create index owner_links on links(user_id,status,created_at desc);
create index click_range on link_clicks(link_id,clicked_at desc);
create index click_owner on link_clicks(user_id,clicked_at desc);
create index site_range on web_events(site_id,occurred_at desc);
create index due_monitor on monitors(next_check_at) where enabled;
create index check_history on monitor_checks(monitor_id,checked_at desc);
create index incident_history on incidents(monitor_id,started_at desc);
create index purge_clicks on link_clicks(clicked_at);
create index purge_events on web_events(received_at);
create index purge_checks on monitor_checks(checked_at);
