package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/usage"
)

// turnedAway is Antigravity's answer to Claude Code's and the Agent SDK's
// system prompt: a 429 that reads as a used-up plan whatever the quota
// left, on every model (#666: Claude Desktop's chats always 429 while its
// titles and Codex on the same account answer). magpie says why, and now
// also leaves the account alone, since nothing is wrong with it.
const turnedAway = `{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`

// claudeCodeSystem is Claude Code's, as 2.1.288 sends it: its billing line
// or the SDK's identity line anywhere in the system instruction.
const claudeCodeSystem = "x-anthropic-billing-header: cc_version=2.1.288.e3f; cc_entrypoint=sdk-cli;\nYou are Claude Code, Anthropic's official CLI for Claude."

// antigravityTurnedAwayGroup is a routing group whose first member holds the
// Antigravity accounts signed in at users, and whose second — "other", another
// provider altogether, not a mate of Antigravity's — answers anything, so the
// first has someone else to fail over to. say is what those accounts' upstream
// answers. The second value counts how often an account's usage was asked of
// the vendor again.
func antigravityTurnedAwayGroup(t *testing.T, users []string, say func(w http.ResponseWriter, r *http.Request)) (*Server, *int) {
	t.Helper()
	fresh(t)
	var logins []map[string]any
	for _, user := range users {
		logins = append(logins, map[string]any{
			"agent": "antigravity", "user": user, "on": true,
			"auth": map[string]any{"access_token": user, "refresh_token": "ref-" + user, "project": "p1",
				"expiry_date": time.Now().Add(time.Hour).UnixMilli()},
		})
	}
	dir := filepath.Dir(provider.Path())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logins.json"), mustJSON(logins), 0o600); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: countTransport(func(r *http.Request) (*http.Response, error) {
		var body string
		switch {
		case strings.HasSuffix(r.URL.Path, "latest-arm64-mac.yml"):
			body = "version: 2.9.1\n"
		case strings.Contains(r.URL.Path, ":loadCodeAssist"):
			body = `{"currentTier":{"id":"standard-tier"},"cloudaicompanionProject":"p1"}`
		case strings.Contains(r.URL.Path, ":fetchAvailableModels"):
			body = `{"models":{"m":{}}}`
		default:
			return nil, fmt.Errorf("unexpected request to %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = old })
	p, err := provider.Find("antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Fetch(t.Context()); err != nil {
		t.Fatal(err)
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, sse(
			`event: message_start`+"\n"+`data: {"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"m1","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`,
			`event: content_block_start`+"\n"+`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`event: content_block_delta`+"\n"+`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"from the other"}}`,
			`event: content_block_stop`+"\n"+`data: {"type":"content_block_stop","index":0}`,
			`event: message_delta`+"\n"+`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
			`event: message_stop`+"\n"+`data: {"type":"message_stop"}`))
	}))
	t.Cleanup(other.Close)
	if err := provider.Save(provider.Provider{ID: "other", Name: "Other", Key: "k", Models: []string{"m"}, Anthropic: other.URL}); err != nil {
		t.Fatal(err)
	}
	if err := provider.SaveGroup(provider.Group{Name: "G", Members: []string{"antigravity/m", "other/m"}, Routing: provider.Ordered}); err != nil {
		t.Fatal(err)
	}
	askedUsage := new(int)
	oldAsk := staleAllowance
	staleAllowance = func(agent, user string) { *askedUsage++ }
	t.Cleanup(func() { staleAllowance = oldAsk })
	s := New()
	// the Antigravity accounts' own upstream, which only the gateway asks;
	// any other member is answered by its own test server
	s.client = &http.Client{Transport: countTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Host, "googleapis.com") {
			return http.DefaultTransport.RoundTrip(r)
		}
		rec := httptest.NewRecorder()
		say(rec, r)
		return rec.Result(), nil
	})}
	return s, askedUsage
}

// claudeTurn is a turn as Claude Code sends it on the group, with system
// as the system instruction the client asked for.
func claudeTurn(system string) string {
	return `{"model":"group/g","max_tokens":100,"system":` + quote(system) + `,"messages":[{"role":"user","content":"hi"}]}`
}

// turnOn posts a turn to this gateway and says what came back.
func turnOn(t *testing.T, s *Server, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)))
	return rec.Code, rec.Body.String()
}

// the account Antigravity turned away is healthy: it rests no quarter of
// an hour for a request it refused, its usage isn't asked again, and the
// next turn asks it first again — as the refusal it is, not a spent quota.
func TestAntigravityTurnedAwayRestsNobody(t *testing.T) {
	s, askedUsage := antigravityTurnedAwayGroup(t, []string{"u@example.com"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, turnedAway)
	})
	code, out := turnOn(t, s, claudeTurn(claudeCodeSystem))
	if code != 200 || !strings.Contains(out, "from the other") {
		t.Fatalf("the turn: %d %s", code, out)
	}
	r := lastRoute(s)
	if len(r.Tries) != 2 || r.Tries[0].Status != 429 || r.Tries[0].Fail != failRefused || r.Tries[0].Rest != nil {
		t.Fatalf("the turned-away try: %+v", r.Tries)
	}
	if !strings.Contains(r.Tries[0].Error, antigravityTurnedAwayHint) {
		t.Fatalf("the attempt isn't told why: %s", r.Tries[0].Error)
	}
	if *askedUsage != 0 {
		t.Fatalf("the account's usage was asked again %d times for a request it refused", *askedUsage)
	}
	if _, ok := restOf("antigravity@u@example.com"); ok {
		t.Fatalf("antigravity@u@example.com rests for a request it answered with a refusal: %v", rests())
	}
	// the same account is asked first again on the next turn
	code, out = turnOn(t, s, claudeTurn(claudeCodeSystem))
	if code != 200 || !strings.Contains(out, "from the other") {
		t.Fatalf("the next turn: %d %s", code, out)
	}
	if r := lastRoute(s); len(r.Tries) != 2 || r.Tries[0].ID != "antigravity" || r.Tries[0].Status != 429 {
		t.Fatalf("the next turn asked %+v first", r.Tries)
	}
}

// A 429 that really is the plan used up — no such system prompt asking —
// still rests the account, as it did (#147, #530).
func TestAntigravityQuotaStillRests(t *testing.T) {
	s, askedUsage := antigravityTurnedAwayGroup(t, []string{"u@example.com"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, turnedAway)
	})
	code, out := turnOn(t, s, claudeTurn("You are a helpful assistant."))
	if code != 200 || !strings.Contains(out, "from the other") {
		t.Fatalf("the turn: %d %s", code, out)
	}
	r := lastRoute(s)
	if len(r.Tries) != 2 || r.Tries[0].Status != 429 || r.Tries[0].Fail != failQuota || r.Tries[0].Rest == nil {
		t.Fatalf("the used-up try: %+v", r.Tries)
	}
	if d := time.Until(r.Tries[0].Rest.Until); d < 14*time.Minute || d > 15*time.Minute {
		t.Fatalf("rests %v, not the 15 minutes a used-up plan takes", d)
	}
	if *askedUsage != 1 {
		t.Fatalf("the account's usage was asked again %d times, once as it was", *askedUsage)
	}
	if _, ok := restOf("antigravity@u@example.com"); !ok {
		t.Fatal("antigravity@u@example.com doesn't rest for a plan used up")
	}
}

// Only the refusal itself leaves the account alone. A Claude Code turn is
// no excuse for an account that really is broken, overloaded or denied: it
// rests as it would on any other request (the maintenance review of #873).
func TestAntigravityTurnedAwayIsOnlyTheRefusal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		fail   string
	}{
		{"500 INTERNAL", http.StatusInternalServerError, `{"error":{"code":500,"message":"internal error","status":"INTERNAL"}}`, failOther},
		{"503 UNAVAILABLE", http.StatusServiceUnavailable, `{"error":{"code":503,"message":"unavailable","status":"UNAVAILABLE"}}`, failOther},
		{"403 PERMISSION_DENIED", http.StatusForbidden, `{"error":{"code":403,"message":"denied","status":"PERMISSION_DENIED"}}`, failOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := antigravityTurnedAwayGroup(t, []string{"u@example.com"}, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			code, out := turnOn(t, s, claudeTurn(claudeCodeSystem))
			if code != 200 || !strings.Contains(out, "from the other") {
				t.Fatalf("the turn: %d %s", code, out)
			}
			r := lastRoute(s)
			if len(r.Tries) != 2 || r.Tries[0].Status != tc.status || r.Tries[0].Fail != tc.fail || r.Tries[0].Rest == nil {
				t.Fatalf("a %d on a Claude Code turn isn't the refusal: %+v", tc.status, r.Tries)
			}
			if _, ok := restOf("antigravity@u@example.com"); !ok {
				t.Fatalf("antigravity@u@example.com doesn't rest for a %d, as it wouldn't on any other request", tc.status)
			}
		})
	}
}

// The refusal costs the account nothing, but it still happened, and the
// ledger keeps it: a Claude Code turn's 429 is as visible as any other
// failed call (the maintenance review of #873).
func TestAntigravityTurnedAwayStillLandsInTheLedger(t *testing.T) {
	s, _ := antigravityTurnedAwayGroup(t, []string{"u@example.com"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, turnedAway)
	})
	if code, out := turnOn(t, s, claudeTurn(claudeCodeSystem)); code != 200 || !strings.Contains(out, "from the other") {
		t.Fatalf("the turn: %d %s", code, out)
	}
	rows, _, _ := usage.Ledger(usage.All, usage.Filter{Provider: "antigravity"})
	if len(rows) == 0 {
		t.Fatal("the refusal is in no usage row")
	}
	for _, row := range rows {
		if row.Status == http.StatusTooManyRequests && row.Failed() {
			return
		}
	}
	t.Fatalf("no 429 row of the refusal among %d: %+v", len(rows), rows)
}

// Its mates are asked last, not first: they carry the same system prompt,
// so each of them is turned away just the same, and asking them first costs
// a round-trip apiece before the group reaches the member that answers
// (the maintenance review of #873).
func TestAntigravityTurnedAwayLeavesItsMatesForLast(t *testing.T) {
	s, _ := antigravityTurnedAwayGroup(t, []string{"u@example.com", "v@example.com"}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, turnedAway)
	})
	if code, out := turnOn(t, s, claudeTurn(claudeCodeSystem)); code != 200 || !strings.Contains(out, "from the other") {
		t.Fatalf("the turn: %d %s", code, out)
	}
	r := lastRoute(s)
	if len(r.Tries) != 2 {
		t.Fatalf("asked %d times: the second account of the member was asked before the one that answers: %+v", len(r.Tries), r.Tries)
	}
	if r.Tries[0].ID != "antigravity" || r.Tries[0].Status != 429 || r.Tries[1].ID != "other" {
		t.Fatalf("asked: %+v", r.Tries)
	}
	for _, user := range []string{"u@example.com", "v@example.com"} {
		if _, ok := restOf("antigravity@" + user); ok {
			t.Fatalf("antigravity@%s rests for a request it was turned away from", user)
		}
	}
}

// The refusal also comes said inside the reply: a 200 that opens with the
// same error event, where the Code Assist decoder keeps only its message.
// It leaves the account alone as the 429 does, and an error event of any
// other kind still rests it.
func TestAntigravityTurnedAwaySaidInsideTheReply(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		rest bool
	}{
		{"the refusal", `data: {"error":{"message":"Resource has been exhausted (e.g. check quota)."}}`, false},
		{"another error", `data: {"error":{"message":"api key not valid"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := antigravityTurnedAwayGroup(t, []string{"u@example.com"}, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, sse(tc.body))
			})
			if code, out := turnOn(t, s, claudeTurn(claudeCodeSystem)); code != 200 || !strings.Contains(out, "from the other") {
				t.Fatalf("the turn: %d %s", code, out)
			}
			r := lastRoute(s)
			if len(r.Tries) != 2 || r.Tries[1].Status != 200 {
				t.Fatalf("the turn: %+v", r.Tries)
			}
			_, resting := restOf("antigravity@u@example.com")
			if resting != tc.rest {
				t.Fatalf("rests %v, want %v: %+v", resting, tc.rest, r.Tries)
			}
			if want := failRefused; !tc.rest && r.Tries[0].Fail != want {
				t.Fatalf("the refusal said inside the reply: Fail=%q: %+v", r.Tries[0].Fail, r.Tries)
			}
		})
	}
}

// rests is what rests now, to say it when nothing should.
func rests() map[string]time.Time {
	restingUntil.Lock()
	defer restingUntil.Unlock()
	out := map[string]time.Time{}
	for k, v := range restingUntil.m {
		if v.After(time.Now()) {
			out[k] = v
		}
	}
	return out
}
