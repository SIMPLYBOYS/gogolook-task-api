package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// drainTimeout caps how long in-flight requests get to finish after a signal.
const drainTimeout = 10 * time.Second

func main() {
	ln, err := net.Listen("tcp", ":"+envOr("PORT", "8080"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Go's zero-value Server has no timeouts, so a client that opens a connection and
	// never finishes its request holds a goroutine indefinitely (Slowloris).
	srv := &http.Server{
		Handler:           newRouter(NewStore()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("task-api listening on %s", ln.Addr())
	if err := serve(ctx, srv, ln); err != nil {
		log.Fatal(err)
	}
}

// serve runs srv until ctx is cancelled, then stops accepting and lets in-flight
// requests drain. Takes a listener so tests can drive it on an ephemeral port.
func serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
