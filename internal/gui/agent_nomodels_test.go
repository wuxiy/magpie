package gui

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tystem on Discord: connecting Codex with no provider or subscription in
// magpie answered "Codex can't be connected to magpie", untranslated. The
// answer carries code no_models and the agent's name, which the GUI says
// in the reader's language.
func TestAgentConnectNoModels(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("model = \"gpt-5.5\"\n"), 0o600)
	rec := httptest.NewRecorder()
	Handler(nil, nil).ServeHTTP(rec, httptest.NewRequest("POST", "/api/agents/connect/codex", strings.NewReader("{}")))
	var out map[string]string
	json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 400 || out["code"] != "no_models" || out["agent"] != "Codex" || !strings.Contains(out["error"], "Add a provider or subscription") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
