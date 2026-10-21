package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"safecircle/internal/httpapi"
	"safecircle/internal/live"
	"safecircle/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbURL := env("DATABASE_URL", "postgres://circle:circle@127.0.0.1:5432/circle?sslmode=disable")
	pool, err := store.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("postgres: %v", err)
	}
	st := store.New(pool)
	if err := st.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	var liveClient *live.Live
	if client, err := live.Open(env("REDIS_URL", "redis://127.0.0.1:6379/0")); err != nil {
		log.Printf("redis unavailable, live updates are off: %v", err)
	} else {
		liveClient = client
		defer liveClient.Close()
	}

	srv := httpapi.New(httpapi.Options{
		Store:     st,
		Live:      liveClient,
		UploadDir: env("UPLOAD_DIR", "data/ids"),
	})
	go srv.Listen(ctx)
	go expireLoop(ctx, srv)

	addr := env("ADDR", ":43123")
	httpServer := &http.Server{Addr: addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("circle api listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func expireLoop(ctx context.Context, srv *httpapi.Server) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.Expire(ctx)
		}
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
