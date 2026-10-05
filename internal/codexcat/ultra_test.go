package codexcat

import (
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// A model offering Codex's Ultra that no ChatGPT account answers for
// (Copilot's gpt-6.1-sol, #656) says multi-agent V2, as Codex's own entry
// for the model does: Ultra hands work to Codex's agents in V2 alone. The
// "subagents on any model" setting (V1) still stamps the OpenAI entries
// alone.
func TestUltraEntriesSayV2(t *testing.T) {
	v1Home(t, `{"etag":"W/\"a\"","models":[]}`)
	ms := []catalog.Model{
		{ID: "copilot/gpt-6.1-sol", Name: "GPT-6.1 Sol", Efforts: []string{"low", "high", "max", "ultra"}, AgentsV2: true},
		{ID: "copilot/gpt-6-luna", Name: "GPT-6 Luna", Efforts: []string{"low", "high", "max"}},
		{ID: "group/sol", Name: "sol", Efforts: []string{"low", "high", "max", "ultra"}, Fast: true},
	}
	for _, on := range []bool{false, true} {
		setV1(t, on)
		got := versions(t, ms)
		want := map[bool]any{false: nil, true: "v1"}[on]
		if got["copilot/gpt-6.1-sol"] != "v2" || got["copilot/gpt-6-luna"] != nil || got["group/sol"] != want {
			t.Fatalf("V1 setting %v: %v", on, got)
		}
	}
}
