package gui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// The tray panel's Allowances tab leaves out the subscriptions set hidden
// there (H20 on Discord), kept beside the Usage page's order: each is saved
// on its own, the other left as it was, and both outlast a save of the
// Settings page.
func TestUsageArrangePanelHidden(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
	t.Setenv("APPDATA", filepath.Join(h, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(h, "AppData", "Local"))
	post := func(path, body string) {
		t.Helper()
		rec := httptest.NewRecorder()
		Handler(nil, nil).ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	want := func(order, hidden []string) {
		t.Helper()
		s := settings.Load()
		if !slices.Equal(s.UsageOrder, order) || !slices.Equal(s.PanelUsageHidden, hidden) {
			t.Fatalf("order %v hidden %v, want %v %v", s.UsageOrder, s.PanelUsageHidden, order, hidden)
		}
	}
	post("/api/usage/arrange", `{"order":["kimi","codex"]}`)
	want([]string{"kimi", "codex"}, nil)
	post("/api/usage/arrange", `{"panelHidden":["deepseek"," deepseek","claude"]}`)
	want([]string{"kimi", "codex"}, []string{"deepseek", "claude"})
	post("/api/usage/arrange", `{"order":["codex","kimi"]}`)
	want([]string{"codex", "kimi"}, []string{"deepseek", "claude"})
	post("/api/settings", `{"theme":"dark"}`)
	want([]string{"codex", "kimi"}, []string{"deepseek", "claude"})
	post("/api/usage/arrange", `{"panelHidden":[]}`)
	want([]string{"codex", "kimi"}, nil)
}
