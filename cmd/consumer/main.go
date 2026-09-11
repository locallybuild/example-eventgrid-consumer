// Command consumer runs an HTTP server that receives Azure EventGrid webhook
// deliveries and logs each event with its arrival time.
package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/locallybuild/example-eventgrid-consumer/internal/consumer"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", envOr("CONSUMER_ADDR", ":8080"), "listen address")
	path := flag.String("path", envOr("CONSUMER_PATH", "/events"), "webhook path (must start with /)")
	tlsCert := flag.String("tls-cert", os.Getenv("CONSUMER_TLS_CERT"), "TLS certificate file; enables HTTPS when set with -tls-key")
	tlsKey := flag.String("tls-key", os.Getenv("CONSUMER_TLS_KEY"), "TLS key file; enables HTTPS when set with -tls-cert")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	h := consumer.New(log, os.Stdout)

	mux := http.NewServeMux()
	mux.Handle(*path, h)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	srv := &http.Server{Addr: *addr, Handler: mux}

	// Azure EventGrid requires https webhook endpoints, so serve TLS when a
	// cert/key pair is provided. Without them, fall back to plain HTTP (useful
	// for local testing with curl).
	tls := *tlsCert != "" && *tlsKey != ""

	go func() {
		scheme := "http"
		if tls {
			scheme = "https"
		}
		log.Info("consumer listening", "scheme", scheme, "addr", *addr, "path", *path)
		var err error
		if tls {
			err = srv.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}
