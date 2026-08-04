package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// A request already being served must finish after the shutdown signal, and
// serve must return once it has.
func TestServeDrainsInFlightRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(100 * time.Millisecond)
		io.WriteString(w, "done")
	})}

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serve(ctx, srv, ln) }()

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			body <- "request failed: " + err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()

	<-started // shut down mid-request
	cancel()

	if got := <-body; got != "done" {
		t.Fatalf("in-flight request was cut off: %q", got)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not return after shutdown")
	}
}
