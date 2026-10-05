package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

// What a restart would cut short (#577): a reply streaming counts until
// it has ended, a GET (the model list) never does, and a Claude Code turn
// waiting on its caller's tool results counts until it has them or ends.
func TestBusy(t *testing.T) {
	fresh(t)
	started, release := make(chan struct{}), make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, `data: {"id":"c","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"hel"}}]}`+"\n\n")
		w.(http.Flusher).Flush()
		close(started)
		<-release
		io.WriteString(w, `data: {"id":"c","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	defer up.Close()
	if err := provider.Save(provider.Provider{ID: "up", Name: "Up", Key: "k", Chat: up.URL + "/v1", Models: []string{"m"}}); err != nil {
		t.Fatal(err)
	}
	s := New()
	gw := httptest.NewServer(s.Handler())
	defer gw.Close()
	if b := s.Busy(); b.Any() || !b.Last.IsZero() {
		t.Fatalf("before any request: %+v", b)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		res, err := http.Post(gw.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"up/m","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
		if err == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
	}()
	<-started
	if res, err := http.Get(gw.URL + "/v1/models"); err == nil {
		res.Body.Close()
	}
	if b := s.Busy(); b.Requests != 1 || b.Tools != 0 || !b.Any() {
		t.Fatalf("streaming: %+v, want one request", b)
	}
	close(release)
	<-done
	deadline := time.Now().Add(2 * time.Second)
	for s.Busy().Requests != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if b := s.Busy(); b.Any() || time.Since(b.Last) > 5*time.Second {
		t.Fatalf("after the stream: %+v", b)
	}

	// a Claude Code turn parked on its caller's tools
	run := &subscriptionRun{bridge: s.subscription, token: "t", pending: map[string]chan mcpToolResult{}}
	s.subscription.mu.Lock()
	s.subscription.runs["t"] = run
	s.subscription.mu.Unlock()
	if b := s.Busy(); b.Tools != 0 {
		t.Fatalf("a run not waiting: %+v", b)
	}
	run.park()
	if b := s.Busy(); b.Tools != 1 || !b.Any() {
		t.Fatalf("a run waiting on tool results: %+v", b)
	}
	run.finish()
	if b := s.Busy(); b.Tools != 0 {
		t.Fatalf("a run ended: %+v", b)
	}
}
