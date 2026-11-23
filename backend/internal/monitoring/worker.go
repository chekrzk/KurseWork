package monitoring

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

type Job struct {
	ID, Lease, URL, Method string
	Timeout, Min, Max      int
	Scheduled              time.Time
}
type Engine struct {
	DB    *pgxpool.Pool
	Check func(context.Context, Job) Result
}

func (e Engine) claim(ctx context.Context) (j Job, err error) {
	err = e.DB.QueryRow(ctx, `with due as(select id from monitors where enabled and next_check_at<=now() and (lease_until is null or lease_until<=now()) order by next_check_at,id limit 1 for update skip locked)
 update monitors m set lease=gen_random_uuid(),lease_until=now()+interval '45 seconds' from due where m.id=due.id returning m.id,m.lease,m.url,m.method,m.timeout_ms,m.status_min,m.status_max,m.next_check_at`).Scan(&j.ID, &j.Lease, &j.URL, &j.Method, &j.Timeout, &j.Min, &j.Max, &j.Scheduled)
	return
}
func (e Engine) finish(ctx context.Context, j Job, r Result) error {
	tx, err := e.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var fails int
	err = tx.QueryRow(ctx, "select failures from monitors where id=$1 and enabled and lease=$2 and lease_until>now() and next_check_at=$3 for update", j.ID, j.Lease, j.Scheduled).Scan(&fails)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "insert into monitor_checks(monitor_id,scheduled_at,http_status,latency_ms,success,error_type) values($1,$2,$3,$4,$5,$6)", j.ID, j.Scheduled, r.Code, r.Latency, r.Success, r.Error)
	if err != nil {
		return err
	}
	state := "operational"
	if r.Success {
		fails = 0
		_, err = tx.Exec(ctx, "update incidents set ended_at=now() where monitor_id=$1 and ended_at is null", j.ID)
	} else {
		fails++
		state = "degraded"
		if fails >= 2 {
			state = "down"
			_, err = tx.Exec(ctx, "insert into incidents(monitor_id,cause) values($1,$2) on conflict(monitor_id) where ended_at is null do nothing", j.ID, r.Error)
		}
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `update monitors set failures=$2,status=$3,last_checked_at=now(),last_success_at=case when $4 then now() else last_success_at end,lease=null,lease_until=null,next_check_at=now()+make_interval(secs=>interval_seconds)+random()*interval '3 seconds' where id=$1`, j.ID, fails, state, r.Success)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (e Engine) Run(ctx context.Context, n int) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				j, err := e.claim(ctx)
				if err == nil {
					r := e.Check(ctx, j)
					if ctx.Err() != nil {
						return
					}
					finish, cancel := context.WithTimeout(ctx, 5*time.Second)
					err = e.finish(finish, j, r)
					cancel()
					if err == nil {
						continue
					}
				}
				if err != nil && !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
					log.Warn().Msg("monitor scheduler database error")
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
			}
		}()
	}
	wg.Wait()
}
