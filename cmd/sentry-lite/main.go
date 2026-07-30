package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/skndan/sentry-lite/internal/alerts"
	"github.com/skndan/sentry-lite/internal/api"
	"github.com/skndan/sentry-lite/internal/bus"
	"github.com/skndan/sentry-lite/internal/config"
	"github.com/skndan/sentry-lite/internal/ingest"
	"github.com/skndan/sentry-lite/internal/process"
	"github.com/skndan/sentry-lite/internal/store"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	b, err := bus.New(cfg.RedpandaBrokers, cfg.IngestTopic)
	if err != nil {
		log.Fatalf("bus: %v", err)
	}
	defer b.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	alertDisp := &alerts.Dispatcher{
		Store:   st,
		APIBase: cfg.PublicURL,
		SMTP:    cfg.AlertSMTP,
		From:    cfg.AlertFrom,
	}

	worker := &process.Worker{
		Store:   st,
		Bus:     b,
		DataDir: cfg.DataDir,
		Alerts:  alertDisp,
	}
	go worker.Run(ctx)

	rollup := &process.RollupWorker{Store: st}
	go rollup.Run(ctx)

	cronWatch := &process.CronWatcher{Store: st, Alerts: alertDisp}
	go cronWatch.Run(ctx)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.CORSOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	ingestHandler := &ingest.Handler{Store: st, Bus: b}
	ingestHandler.Routes(r)

	apiHandler := &api.Handler{Store: st, PublicURL: cfg.PublicURL}
	apiHandler.Routes(r)

	serveSPA(r, cfg.WebDist)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		log.Printf("seed DSN: http://%s@localhost%s/1", store.SeedPublicKey, normalizePort(cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func normalizePort(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}

func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowed[origin] || allowed["*"]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Sentry-Auth, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func serveSPA(r chi.Router, dist string) {
	abs, err := filepath.Abs(dist)
	if err != nil {
		log.Printf("web dist: %v", err)
		return
	}
	if _, err := os.Stat(abs); err != nil {
		log.Printf("web dist not found at %s (UI will be unavailable until built)", abs)
		return
	}
	fileServer := http.FileServer(http.Dir(abs))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := filepath.Join(abs, filepath.Clean(req.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, req)
			return
		}
		http.ServeFile(w, req, filepath.Join(abs, "index.html"))
	})
}
