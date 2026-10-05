package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// strictRelay checks the request as CHCP and Keenc's Vertex relay do
// (pydantic, extra fields forbidden): thinking.display only summarized or
// omitted, and nothing but role and content on a message.
type strictRelay struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func (v *strictRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	json.Unmarshal(b, &m)
	v.mu.Lock()
	v.bodies = append(v.bodies, m)
	v.mu.Unlock()
	var errs []string
	th, _ := m["thinking"].(map[string]any)
	if d, ok := th["display"]; ok && d != "summarized" && d != "omitted" {
		errs = append(errs, fmt.Sprintf("thinking.%s.display: Input should be 'summarized', 'omitted'", th["type"]))
	}
	msgs, _ := m["messages"].([]any)
	for i, x := range msgs {
		for k := range x.(map[string]any) {
			if k != "role" && k != "content" {
				errs = append(errs, fmt.Sprintf("messages.%d.%s: Extra inputs are not permitted", i, k))
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if len(errs) > 0 {
		w.WriteHeader(400)
		e, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]any{"type": "invalid_request_error", "message": strings.Join(errs, "; ")}})
		w.Write(e)
		return
	}
	io.WriteString(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
}

func (v *strictRelay) all() []map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.bodies)
}

// Keenc on Discord: Claude Code 2.1.288 through magpie to a relay got
// "thinking.adaptive.display: Input should be 'summarized', 'omitted'" (its
// display "updates") and "messages.1.output_config: Extra inputs are not
// permitted" (a turn's effort on a system message). magpie asks the relay
// again without them, and from then on sends it none; the request's own
// effort and a system message's text stay.
func TestRelayRefusingClaudeCodesNewShapes(t *testing.T) {
	fresh(t)
	up := &strictRelay{}
	srv := httptest.NewServer(up)
	t.Cleanup(srv.Close)
	if err := provider.Save(provider.Provider{ID: "chcp", Name: "CHCP", Key: "k", Anthropic: srv.URL,
		Models: []string{"claude-opus-5.5"}}); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetUpstreamName("chcp/claude-opus-5.5", "claude-opus-5-5"); err != nil {
		t.Fatal(err)
	}
	// as Claude Code sends it: an environment system message after the
	// first user message, and the second turn's effort on an empty one
	body := `{"model":"chcp/claude-opus-5.5","max_tokens":64000,` +
		`"thinking":{"type":"adaptive","display":"updates"},"output_config":{"effort":"low"},"messages":[` +
		`{"role":"user","content":[{"type":"text","text":"say pong"}]},` +
		`{"role":"system","content":[{"type":"text","text":"# Environment <cwd>"}],"output_config":{"effort":"high"}},` +
		`{"role":"assistant","content":[{"type":"text","text":"pong"}]},` +
		`{"role":"user","content":[{"type":"text","text":"again"}]},` +
		`{"role":"system","content":[],"output_config":{"effort":"low"}}]}`
	check := func(t *testing.T, got map[string]any) {
		t.Helper()
		if th, _ := json.Marshal(got["thinking"]); string(th) != `{"type":"adaptive"}` {
			t.Errorf("thinking = %s, want display left out", th)
		}
		if oc, _ := json.Marshal(got["output_config"]); string(oc) != `{"effort":"low"}` {
			t.Errorf("output_config = %s, want the request's own", oc)
		}
		msgs, _ := got["messages"].([]any)
		var roles []string
		for _, x := range msgs {
			m := x.(map[string]any)
			roles = append(roles, m["role"].(string))
			if _, ok := m["output_config"]; ok {
				t.Errorf("a message kept its output_config: %v", m)
			}
		}
		if r := strings.Join(roles, ","); r != "user,system,assistant,user" {
			t.Errorf("roles = %s, want the empty system message dropped and the environment kept", r)
		}
		if c, _ := msgs[1].(map[string]any)["content"].([]any); len(c) != 1 || c[0].(map[string]any)["text"] != "# Environment <cwd>" {
			t.Errorf("system message content = %v", c)
		}
	}

	h := New().Handler() // one gateway, which learns
	post := func(body string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)))
		return rec.Code, rec.Body.String()
	}
	code, out := post(body)
	if code != 200 {
		t.Fatalf("status %d: %s", code, out)
	}
	sent := up.all()
	if len(sent) < 2 {
		t.Fatalf("relay asked %d times, want again after its 400", len(sent))
	}
	check(t, sent[len(sent)-1])

	// learned: the next request goes as the relay takes it, the first time
	n := len(sent)
	if code, out = post(body); code != 200 {
		t.Fatalf("status %d: %s", code, out)
	}
	if sent = up.all(); len(sent) != n+1 {
		t.Fatalf("relay asked %d more times, want once", len(sent)-n)
	}
	check(t, sent[len(sent)-1])
}

// A provider that takes them (Anthropic's own API, a relay passing them
// on) is sent them as Claude Code wrote them; "highlights" goes as
// "summarized" to one that takes only the two.
func TestClaudeCodesNewShapesKeptWhereTaken(t *testing.T) {
	fresh(t)
	up := &adaptiveVendor{}
	srv := httptest.NewServer(up)
	t.Cleanup(srv.Close)
	if err := provider.Save(provider.Provider{ID: "anth", Name: "Anth", Key: "k", Anthropic: srv.URL,
		Models: []string{"claude-opus-5-5"}}); err != nil {
		t.Fatal(err)
	}
	body := `{"model":"anth/claude-opus-5-5","max_tokens":64000,"thinking":{"type":"adaptive","display":"updates"},"messages":[` +
		`{"role":"user","content":"hi"},{"role":"system","content":[],"output_config":{"effort":"high"}}]}`
	if code, out := post(t, "/v1/messages", body); code != 200 {
		t.Fatalf("status %d: %s", code, out)
	}
	got := up.last()
	if th, _ := json.Marshal(got["thinking"]); string(th) != `{"display":"updates","type":"adaptive"}` {
		t.Errorf("thinking = %s, want as sent", th)
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 2 || msgs[1].(map[string]any)["output_config"] == nil {
		t.Errorf("messages = %v, want as sent", msgs)
	}

	if got := withDisplay([]byte(`{"thinking":{"type":"adaptive","display":"highlights"}}`), false); string(got) != `{"thinking":{"display":"summarized","type":"adaptive"}}` {
		t.Errorf("highlights = %s", got)
	}
	if got := withDisplay([]byte(`{"thinking":{"type":"adaptive","display":"omitted"}}`), false); string(got) != `{"thinking":{"type":"adaptive","display":"omitted"}}` {
		t.Errorf("omitted = %s, want kept", got)
	}
}
