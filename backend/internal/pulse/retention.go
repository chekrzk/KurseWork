package pulse

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"time"
)

// Delete at most one bounded batch per stream per tick, without locking live events.
func Retention(ctx context.Context, db *pgxpool.Pool) {
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for _, q := range []string{
				"delete from monitor_checks where id in (select id from monitor_checks where checked_at<now()-interval '90 days' order by checked_at limit 1000 for update skip locked)",
				"delete from link_clicks where id in (select id from link_clicks where occurred_at<now()-interval '180 days' order by occurred_at limit 1000 for update skip locked)",
				"delete from web_events where (site_id,id) in (select site_id,id from web_events where received_at<now()-interval '180 days' order by received_at limit 1000 for update skip locked)",
			} {
				batch, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := db.Exec(batch, q)
				cancel()
				if err != nil && ctx.Err() == nil {
					log.Warn().Msg("event retention batch failed")
				}
			}
		}
	}
}
