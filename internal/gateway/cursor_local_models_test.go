package gateway

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// Cursor Private Inference reads magpie's /models for each model's APIs
// (api_types: it asks Anthropic Messages only when no OpenAI one is
// listed) and its limits (capabilities.context_length) (#299): asked with
// its key, each model has them, the ones its provider serves it on; asked
// by any other client, the list is as it was.
func TestCursorLocalModels(t *testing.T) {
	setup(t, provider.Anthropic, &fake{})
	if err := provider.Save(provider.Provider{ID: "ds", Name: "DS", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"flash"}}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("ds", "http://127.0.0.1:1/v1", []catalog.Model{{ID: "flash", Context: 128000, Output: 8192}}); err != nil {
		t.Fatal(err)
	}
	list := func(key string) map[string]map[string]any {
		t.Helper()
		req := httptest.NewRequest("GET", "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		New().Handler().ServeHTTP(rec, req)
		var out struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(rec.Body.String())
		}
		got := map[string]map[string]any{}
		for _, d := range out.Data {
			got[d["id"].(string)] = d
		}
		return got
	}
	types := func(m map[string]any) []string {
		var out []string
		for _, v := range m["api_types"].([]any) {
			out = append(out, v.(string))
		}
		return out
	}

	got := list(TokenFor("cursor-local"))
	f, ds := got["fake/m1"], got["ds/flash"]
	if f == nil || ds == nil {
		t.Fatalf("models: %v", got)
	}
	if ts := types(f); !slices.Equal(ts, []string{"anthropic_messages"}) {
		t.Errorf("an Anthropic model's api_types = %v", ts)
	}
	if ts := types(ds); !slices.Equal(ts, []string{"chat_completions"}) {
		t.Errorf("a Chat model's api_types = %v", ts)
	}
	c, _ := ds["capabilities"].(map[string]any)
	if c["context_length"] != float64(128000) || c["max_output_tokens"] != float64(8192) {
		t.Errorf("capabilities = %v", ds["capabilities"])
	}

	for _, key := range []string{Token, TokenFor("cursor"), TokenFor("opencode")} {
		for id, m := range list(key) {
			if m["api_types"] != nil || m["capabilities"] != nil {
				t.Errorf("%s: %s has %v %v", key, id, m["api_types"], m["capabilities"])
			}
		}
	}

	// the models picked on its row are its picker's: one taken out isn't
	// listed with its key, and still is for another agent's
	if err := provider.SetHiddenModels("cursor-local", []string{"ds/flash"}); err != nil {
		t.Fatal(err)
	}
	if got := list(TokenFor("cursor-local")); got["ds/flash"] != nil || got["fake/m1"] == nil {
		t.Errorf("with ds/flash taken out: %v", got)
	}
	if got := list(TokenFor("opencode")); got["ds/flash"] == nil {
		t.Errorf("taken out of another agent's list: %v", got)
	}
}
