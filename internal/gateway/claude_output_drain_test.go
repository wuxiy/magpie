package gateway

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// What Claude Code writes as it exits reaches the client: its output is
// read to the end before the run ends, not cut by Wait closing the pipe
// under the reader ("read |0: file already closed", CI on 963a8d8f).
func TestClaudeLastWordsAreRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands in for Claude Code")
	}
	dir := t.TempDir()
	// a long reply, more than a pipe holds, written all at once on exit
	script := `#!/bin/sh
read -r line
d='{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"` + strings.Repeat("x", 1000) + `"}}}'
{
echo '{"type":"stream_event","event":{"type":"message_start","message":{"id":"m","model":"claude-sonnet-5","usage":{"input_tokens":1}}}}'
echo '{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}'
i=0; while [ $i -lt 300 ]; do echo "$d"; i=$((i+1)); done
echo '{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"the very end"}}}'
echo '{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}}'
echo '{"type":"stream_event","event":{"type":"message_stop"}}'
}
`
	os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for i := 0; i < 5; i++ {
		s := New()
		p := provider.Provider{ID: "claude", Account: &provider.Account{Agent: "claude", User: "u"}}
		body := `{"model":"claude-sonnet-5","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"say a lot"}]}`
		rec := httptest.NewRecorder()
		var u Usage
		s.serveClaudeSubscription(rec, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)), provider.Anthropic, p, "claude-sonnet-5", []byte(body), &u)
		s.subscription.abortAll()
		out := rec.Body.String()
		if !strings.Contains(out, "the very end") || strings.Contains(out, "event: error") {
			t.Fatalf("run %d: the reply's end: %q", i, out[max(0, len(out)-400):])
		}
	}
}
