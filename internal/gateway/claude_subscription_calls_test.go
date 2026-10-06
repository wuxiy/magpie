package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Claude Code makes a reply's MCP calls one after another, while the client
// runs them together and sends both results at once: the second result
// waits for its call instead of being lost (#84).
func TestResultForACallNotMadeYetWaitsForIt(t *testing.T) {
	b := newSubscriptionBridge()
	run := &subscriptionRun{bridge: b, token: "tok", pending: map[string]chan mcpToolResult{}}
	b.runs["tok"] = run
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cb/{token}", b.mcpCall)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	call := func(id string) chan string {
		out := make(chan string, 1)
		go func() {
			res, err := http.Post(srv.URL+"/cb/tok", "application/json", strings.NewReader(`{"tool_call_id":"`+id+`","name":"bash","arguments":{}}`))
			if err != nil {
				out <- err.Error()
				return
			}
			defer res.Body.Close()
			var r mcpToolResult
			_ = json.NewDecoder(res.Body).Decode(&r)
			out <- r.Content[0]["text"].(string)
		}()
		return out
	}
	first := call("call_1")
	for deadline := time.Now().Add(2 * time.Second); ; {
		b.mu.Lock()
		n := len(b.calls)
		b.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("call_1 was not registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	req := &Request{Messages: []Message{
		{Role: "user", Parts: []Part{{Kind: ToolResult, CallID: "old", Text: "earlier"}}},
		{Role: "assistant", Parts: []Part{{Kind: ToolCall, ID: "call_1"}, {Kind: ToolCall, ID: "call_2"}}},
		{Role: "user", Parts: []Part{{Kind: ToolResult, CallID: "call_1", Text: "one"}, {Kind: ToolResult, CallID: "call_2", Text: "two"}}},
	}}
	found, results := b.findRun(req)
	if found != run || len(results) != 2 {
		t.Fatalf("findRun = %v, %d results", found == run, len(results))
	}
	if _, err := run.continueWith(results, nil); err != nil {
		t.Fatal(err)
	}
	if got := <-first; got != "one" {
		t.Fatalf("call_1 answered %q", got)
	}
	select {
	case got := <-call("call_2"):
		if got != "two" {
			t.Fatalf("call_2 answered %q", got)
		}
	case <-time.After(2 * time.Second):
		run.finish() // lets the waiting call go, so the server can close
		t.Fatal("call_2 never had its result: the turn is stuck")
	}
}

func TestToolResultKeepsUserTextSentMidTurn(t *testing.T) {
	b := newSubscriptionBridge()
	waiter := make(chan mcpToolResult, 1)
	run := &subscriptionRun{
		bridge:  b,
		token:   "tok",
		pending: map[string]chan mcpToolResult{"call_1": waiter},
	}
	b.runs["tok"] = run
	b.calls["call_1"] = run

	req := &Request{Messages: []Message{
		{Role: "assistant", Parts: []Part{{Kind: ToolCall, ID: "call_1", Name: "bash"}}},
		{Role: "user", Parts: []Part{
			{Kind: ToolResult, CallID: "call_1", Text: "tool finished"},
			{Kind: Text, Text: "<system-reminder>also answer the message I sent while the tool was running</system-reminder>"},
		}},
	}}
	found, results := b.findRun(req)
	if found != run || len(results) != 1 {
		t.Fatalf("findRun = %v, %d results", found == run, len(results))
	}
	if _, err := run.continueWith(results, nil); err != nil {
		t.Fatal(err)
	}
	got := <-waiter
	var text strings.Builder
	for _, block := range got.Content {
		if block["type"] == "text" {
			text.WriteString(block["text"].(string))
		}
	}
	if !strings.Contains(text.String(), "also answer the message I sent while the tool was running") {
		t.Fatalf("mid-turn user text was dropped: %q", text.String())
	}
}
