package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// Codex kept signed in to an account of the user's choosing (#524: the
// first is only the one whose allowance is spent first, not the one the
// user uses Codex as): the gateway still tries the first first, In order,
// and the one Codex is signed in to at its own place behind it.
func TestKeptLoginStandsAtItsPlace(t *testing.T) {
	codexSignedIn(t, "spare@example.com")
	var tried []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		tried = append(tried, r.Header.Get("chatgpt-account-id"))
		io.WriteString(w, sse(
			`data: {"type":"response.created","response":{"id":"r1","model":"gpt-5.5"}}`,
			`data: {"type":"response.output_text.delta","delta":"pong"}`,
			`data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":7,"output_tokens":1}}}`))
	}))
	t.Cleanup(up.Close)
	was := provider.CodexBase
	provider.CodexBase = up.URL + "/backend-api/codex"
	t.Cleanup(func() { provider.CodexBase = was })
	if err := provider.SetRouting("codex", provider.Ordered); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetKeepLoginAs("codex", "spare@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, l := range provider.Logins("codex") {
		if l.Active != (l.User == "spare@example.com") {
			t.Fatalf("kept on spare, signed in as %+v", l)
		}
	}
	code, body := codexPost(t, `{"model":"gpt-5.5","stream":true,"input":"ping"}`)
	if code != 200 || strings.Join(tried, ",") != "acct-1" {
		t.Fatalf("%d %s: tried %v, not the first (acct-1)", code, body, tried)
	}
	if got := provider.InUseLogin("codex"); got != "me@example.com" {
		t.Fatalf("in use %q", got)
	}
	// let go, Codex is signed in to the first again, and it goes first
	if err := provider.SetKeepLogin("codex", false); err != nil {
		t.Fatal(err)
	}
	for _, l := range provider.Logins("codex") {
		if l.Active != (l.User == "me@example.com") {
			t.Fatalf("let go, signed in as %+v", l)
		}
	}
	tried = nil
	codexPost(t, `{"model":"gpt-5.5","stream":true,"input":"ping"}`)
	if strings.Join(tried, ",") != "acct-1" {
		t.Fatalf("let go, tried %v", tried)
	}
}
