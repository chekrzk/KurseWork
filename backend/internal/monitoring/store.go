package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

var ErrInput = errors.New("invalid monitor configuration")
var ErrState = errors.New("action unavailable in current state")

type Range struct {
	Min int `json:"min"`
	Max int `json:"max"`
}
type Config struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Kind     string `json:"kind"`
	Method   string `json:"method"`
	Interval int    `json:"interval_seconds"`
	Timeout  int    `json:"timeout_ms"`
	Expected Range  `json:"expected_status"`
}
type Monitor struct {
	Config
	ID      string     `json:"id"`
	Link    *string    `json:"link_id"`
	Enabled bool       `json:"enabled"`
	Status  string     `json:"status"`
	Last    *time.Time `json:"last_checked_at"`
	Success *time.Time `json:"last_success_at"`
	Next    *time.Time `json:"next_check_at"`
}

func Defaults() Config {
	return Config{Kind: "website", Method: "HEAD", Interval: 300, Timeout: 5000, Expected: Range{200, 299}}
}
func (d Config) validate() error {
	if _, e := Target(d.URL); e != nil || strings.TrimSpace(d.Name) == "" || len([]rune(d.Name)) > 160 || (d.Method != "GET" && d.Method != "HEAD") || d.Interval < 300 || d.Interval > 86400 || d.Timeout < 500 || d.Timeout > 30000 || d.Expected.Min < 100 || d.Expected.Max > 599 || d.Expected.Max < d.Expected.Min {
		return ErrInput
	}
	return nil
}

type Store struct{ DB *pgxpool.Pool }

const cols = "id,link_id,name,url,kind,method,interval_seconds,timeout_ms,status_min,status_max,enabled,status,last_checked_at,last_success_at,case when enabled then next_check_at end"

func scan(row pgx.Row) (m Monitor, e error) {
	e = row.Scan(&m.ID, &m.Link, &m.Name, &m.URL, &m.Kind, &m.Method, &m.Interval, &m.Timeout, &m.Expected.Min, &m.Expected.Max, &m.Enabled, &m.Status, &m.Last, &m.Success, &m.Next)
	return
}
func (s Store) Get(ctx context.Context, user, id string) (Monitor, error) {
	return scan(s.DB.QueryRow(ctx, "select "+cols+" from monitors where user_id=$1 and id=$2", user, id))
}
func (s Store) List(ctx context.Context, user string) ([]Monitor, error) {
	rows, e := s.DB.Query(ctx, "select "+cols+" from monitors where user_id=$1 order by created_at desc", user)
	if e != nil {
		return nil, e
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Monitor, error) { return scan(r) })
}
func quota(ctx context.Context, tx pgx.Tx, user string) error {
	var ok bool
	if e := tx.QueryRow(ctx, "select true from users where id=$1 for update", user).Scan(&ok); e != nil {
		return e
	}
	if e := tx.QueryRow(ctx, "select count(*)<50 from monitors where user_id=$1", user).Scan(&ok); e != nil {
		return e
	}
	if !ok {
		return ErrState
	}
	return nil
}
func (s Store) Save(ctx context.Context, user, id string, apply func(*Config) error) (Monitor, error) {
	var empty Monitor
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return empty, e
	}
	defer tx.Rollback(ctx)
	d := Defaults()
	var prev Monitor
	if id != "" {
		prev, e = scan(tx.QueryRow(ctx, "select "+cols+" from monitors where user_id=$1 and id=$2 for update", user, id))
		if e != nil {
			return empty, e
		}
		d = prev.Config
	}
	if e = apply(&d); e != nil {
		return empty, e
	}
	if e = d.validate(); e != nil {
		return empty, e
	}
	if prev.Link != nil {
		if d.URL != prev.URL || d.Kind != "link_target" {
			return empty, ErrInput
		}
	} else if d.Kind != "api" && d.Kind != "website" {
		return empty, ErrInput
	}
	if id == "" {
		if e = quota(ctx, tx, user); e != nil {
			return empty, e
		}
		e = tx.QueryRow(ctx, "insert into monitors(user_id,name,url,kind,method,interval_seconds,timeout_ms,status_min,status_max) values($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id", user, d.Name, d.URL, d.Kind, d.Method, d.Interval, d.Timeout, d.Expected.Min, d.Expected.Max).Scan(&id)
	} else {
		_, e = tx.Exec(ctx, "update monitors set name=$2,url=$3,kind=$4,method=$5,interval_seconds=$6,timeout_ms=$7,status_min=$8,status_max=$9,lease=null,lease_until=null where id=$1", id, d.Name, d.URL, d.Kind, d.Method, d.Interval, d.Timeout, d.Expected.Min, d.Expected.Max)
		if e == nil && prev.URL != d.URL {
			e = reset(ctx, tx, id, prev.Enabled)
		}
	}
	if e != nil {
		return empty, e
	}
	m, e := scan(tx.QueryRow(ctx, "select "+cols+" from monitors where id=$1", id))
	if e != nil {
		return empty, e
	}
	return m, tx.Commit(ctx)
}
func reset(ctx context.Context, tx pgx.Tx, id string, enabled bool) error {
	_, e := tx.Exec(ctx, "update monitors set enabled=$2,status=case when $2 then 'unknown' else 'paused' end,failures=0,lease=null,lease_until=null where id=$1", id, enabled)
	if e == nil {
		_, e = tx.Exec(ctx, "update incidents set ended_at=now() where monitor_id=$1 and ended_at is null", id)
	}
	return e
}
func (s Store) Action(ctx context.Context, user, id, action string) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var enabled, archived bool
	e = tx.QueryRow(ctx, "select m.enabled,coalesce(l.status='archived',false) from monitors m left join links l on l.id=m.link_id where m.user_id=$1 and m.id=$2 for update of m", user, id).Scan(&enabled, &archived)
	if e != nil {
		return e
	}
	switch action {
	case "delete":
		_, e = tx.Exec(ctx, "delete from monitors where id=$1", id)
	case "pause", "resume":
		on := action == "resume"
		if on && archived {
			return ErrState
		}
		if enabled != on {
			e = reset(ctx, tx, id, on)
		}
	case "check":
		tag, err := tx.Exec(ctx, `update monitors set next_check_at=now(),manual_at=now() where id=$1 and enabled and (lease_until is null or lease_until<now()) and (manual_at is null or manual_at<now()-interval '5 minutes')`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrState
		}
	default:
		return ErrInput
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// SyncLink participates in the caller's link transaction. Archiving preserves checks.
func SyncLink(ctx context.Context, tx pgx.Tx, user, link, name, raw, status string, want *bool) error {
	if want != nil && !*want {
		_, e := tx.Exec(ctx, "delete from monitors where user_id=$1 and link_id=$2", user, link)
		return e
	}
	var id, old string
	var enabled bool
	e := tx.QueryRow(ctx, "select id,url,enabled from monitors where user_id=$1 and link_id=$2 for update", user, link).Scan(&id, &old, &enabled)
	if errors.Is(e, pgx.ErrNoRows) {
		if want == nil {
			return nil
		}
		if _, e = Target(raw); e != nil {
			return ErrInput
		}
		if e = quota(ctx, tx, user); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `insert into monitors(user_id,link_id,name,url,kind,method,interval_seconds,timeout_ms,status_min,status_max,enabled,status) values($1,$2,$3,$4,'link_target','HEAD',300,5000,200,299,$5,case when $5 then 'unknown' else 'paused' end)`, user, link, name, raw, status != "archived")
		return e
	}
	if e != nil {
		return e
	}
	if _, e = Target(raw); e != nil {
		return ErrInput
	}
	if old != raw || status == "archived" && enabled {
		if e = reset(ctx, tx, id, enabled && status != "archived"); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "update monitors set name=$1,url=$2 where id=$3", name, raw, id)
	return e
}
func (s Store) Report(ctx context.Context, user, id, view string, from, to time.Time) (json.RawMessage, error) {
	if _, e := s.Get(ctx, user, id); e != nil {
		return nil, e
	}
	var q string
	switch view {
	case "checks":
		q = `select jsonb_build_object('items',coalesce(jsonb_agg(to_jsonb(x) order by checked_at desc),'[]')) from (select id,checked_at,http_status,latency_ms,success,error_type from monitor_checks where monitor_id=$1 and checked_at >=$2 and checked_at<$3 order by checked_at desc limit 200)x`
	case "incidents":
		q = `select jsonb_build_object('items',coalesce(jsonb_agg(to_jsonb(x) order by started_at desc),'[]')) from(select id,started_at,ended_at,cause,case when ended_at is null then 'open' else 'resolved' end status from incidents where monitor_id=$1 and started_at<$3 and (ended_at is null or ended_at>=$2) order by started_at desc limit 200)x`
	case "analytics":
		q = `with samples as(select * from monitor_checks where monitor_id=$1 and checked_at >=$2::timestamptz and checked_at<$3::timestamptz), buckets as(select date_trunc(case when $3::timestamptz-$2::timestamptz<=interval '1 day' then 'hour' else 'day' end,checked_at,'UTC') bucket,count(*) sample_count,avg(success::int)*100 uptime_percent,percentile_cont(0.95) within group(order by latency_ms) p95_latency_ms from samples group by 1)
 select jsonb_build_object('sample_count',count(*),'uptime_percent',avg(success::int)*100,'average_latency_ms',avg(latency_ms),'p95_latency_ms',percentile_cont(0.95) within group(order by latency_ms),'series',(select coalesce(jsonb_agg(to_jsonb(buckets) order by bucket),'[]') from buckets)) from samples`
	default:
		return nil, ErrInput
	}
	var out json.RawMessage
	e := s.DB.QueryRow(ctx, q, id, from, to).Scan(&out)
	return out, e
}
