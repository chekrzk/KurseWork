package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLeasesAndIncidentLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires TEST_DATABASE_URL")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := pgx.Identifier{"test_" + uuid.NewString()}.Sanitize()
	if _, e = admin.Exec(ctx, "create schema "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "drop schema "+schema+" cascade")
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	files, e := filepath.Glob("../../migrations/*.up.sql")
	if e != nil || len(files) < 5 {
		t.Fatal("migration files", e)
	}
	sort.Strings(files)
	for _, f := range files {
		sql, e := os.ReadFile(f)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(sql)); e != nil {
			t.Fatal(e)
		}
	}
	var user string
	if e = db.QueryRow(ctx, "insert into users(email,password_hash,display_name) values('worker@test.invalid','unused','Worker') returning id").Scan(&user); e != nil {
		t.Fatal(e)
	}
	s := Store{db}
	m, e := s.Save(ctx, user, "", func(d *Config) error { d.Name = "Fixture"; d.URL = "https://example.org"; return nil })
	if e != nil {
		t.Fatal(e)
	}
	w := Engine{DB: db}
	j, e := w.claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = w.claim(ctx); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("parallel claim", e)
	}
	if _, e = db.Exec(ctx, "update monitors set lease_until=now()-interval '1 second'"); e != nil {
		t.Fatal(e)
	}
	retry, e := w.claim(ctx)
	if e != nil || retry.Lease == j.Lease {
		t.Fatal("lease retry", e)
	}
	if e = w.finish(ctx, j, Result{Success: true}); e != nil {
		t.Fatal(e)
	}
	errKind := "timeout"
	for i, state := range []string{"degraded", "down", "operational"} {
		if i > 0 {
			if _, e = db.Exec(ctx, "update monitors set next_check_at=now()"); e != nil {
				t.Fatal(e)
			}
			retry, e = w.claim(ctx)
			if e != nil {
				t.Fatal(e)
			}
		}
		r := Result{Error: &errKind, Latency: 10 + i}
		if i == 2 {
			r = Result{Success: true, Latency: 12}
		}
		if e = w.finish(ctx, retry, r); e != nil {
			t.Fatal(e)
		}
		current, e := s.Get(ctx, user, m.ID)
		if e != nil || current.Status != state {
			t.Fatal(current, e)
		}
	}
	var checks, closed int
	e = db.QueryRow(ctx, "select (select count(*) from monitor_checks),(select count(*) from incidents where ended_at is not null)").Scan(&checks, &closed)
	if e != nil || checks != 3 || closed != 1 {
		t.Fatal(checks, closed, e)
	}
	report, e := s.Report(ctx, user, m.ID, "analytics", time.Now().Add(-time.Hour), time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	var stats map[string]json.RawMessage
	if err := json.Unmarshal(report, &stats); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]float64{"sample_count": 3, "average_latency_ms": 11, "p95_latency_ms": 11.9, "uptime_percent": 100.0 / 3} {
		var got float64
		if err := json.Unmarshal(stats[key], &got); err != nil || math.Abs(got-want) > 0.001 {
			t.Errorf("%s: got %s, want %g", key, stats[key], want)
		}
	}
	if e = s.Action(ctx, user, m.ID, "pause"); e != nil {
		t.Fatal(e)
	}
	if _, e = w.claim(ctx); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal(e)
	}
}
