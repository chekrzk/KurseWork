package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"net/url"
	"os"
	"os/signal"
	"saturnita/pulse/internal/httpapi"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/monitoring"
	"saturnita/pulse/internal/pulse"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	_ = godotenv.Load()
	origin := os.Getenv("APP_ORIGIN")
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		log.Fatal().Msg("invalid APP_ORIGIN")
	}
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal().Msg("DATABASE_URL required")
	}
	pool, e := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal().Msg("invalid database configuration")
	}
	defer pool.Close()
	workers := 10
	if raw := os.Getenv("MONITOR_WORKERS"); raw != "" {
		workers, e = strconv.Atoi(raw)
		if e != nil || workers < 1 || workers > 32 {
			log.Fatal().Msg("MONITOR_WORKERS must be 1..32")
		}
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if u.Hostname() != "localhost" && len(os.Getenv("ANALYTICS_MASTER_KEY")) < 32 {
		log.Fatal().Msg("ANALYTICS_MASTER_KEY must contain at least 32 bytes")
	}
	maintenanceDone := make(chan struct{})
	go func() { defer close(maintenanceDone); pulse.Retention(stop, pool) }()
	defer func() { cancel(); <-maintenanceDone }()
	engineDone := make(chan struct{})
	go func() {
		defer close(engineDone)
		monitoring.Engine{DB: pool, Check: monitoring.SafeChecker().Check}.Run(stop, workers)
	}()
	defer func() { cancel(); <-engineDone }()
	app := httpapi.New(identity.New(pool), origin, context.Background())
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	done := make(chan error, 1)
	go func() { done <- app.Listen(addr) }()
	select {
	case e = <-done:
		if e != nil {
			log.Fatal().Msg("listen failed")
		}
	case <-stop.Done():
		if e = app.ShutdownWithTimeout(10 * time.Second); e != nil {
			log.Error().Msg("shutdown deadline")
		}
		<-done
	}
}
