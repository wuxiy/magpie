package access

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestModelPatterns(t *testing.T) {
	ps := []string{"openai/gpt-5", "anthropic/*", "relay/*-mini"}
	for id, want := range map[string]bool{
		"openai/gpt-5":           true,
		"OpenAI/GPT-5":           true,
		"openai/gpt-5-mini":      false,
		"anthropic/claude-x":     true,
		"anthropic/vendor/x":     true,
		"anthropics/x":           false,
		"relay/m-mini":           true,
		"relay/m":                false,
		"openrouter/anthropic/x": false,
	} {
		if got := ModelAllowed(ps, id); got != want {
			t.Errorf("%s: %v", id, got)
		}
	}
	if !ModelAllowed(nil, "any/thing") {
		t.Fatal("no patterns hold a model")
	}
	who := Identity{Models: ps}
	if !who.Restricted() || who.Allows("relay/m", "") || !who.Allows("relay/m", "openai/gpt-5") || (Identity{}).Restricted() || !(Identity{}).Allows("x/y") {
		t.Fatal("Identity.Allows")
	}
	got, err := CleanModels([]string{" openai/gpt-5 ", "", "OPENAI/GPT-5", "anthropic/*"})
	if err != nil || !slices.Equal(got, []string{"openai/gpt-5", "anthropic/*"}) {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"gpt-5", "/gpt-5", "openai/", strings.Repeat("a", 300) + "/b"} {
		if _, err := CleanModels([]string{bad}); err == nil {
			t.Error("accepted", bad)
		}
	}
}

// A key's models are kept in caller-keys.json, stay through a rotation and
// a restart, reach the gateway with the key, and an older file without
// them reads as every model (#882).
func TestKeyModelsKept(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	secret, err := Update("add-key", Change{Name: "Phone"})
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := List()
	id := keys[0].ID
	if who, ok := Authenticate(secret); !ok || who.Restricted() {
		t.Fatal("a new key is held", who)
	}
	if _, err := Update("models-key", Change{Key: id, Models: []string{"gpt-5"}}); err == nil {
		t.Fatal("accepted a model without its provider")
	}
	if _, err := Update("models-key", Change{Key: "missing", Models: []string{"a/b"}}); err == nil {
		t.Fatal("held a missing key")
	}
	if _, err := Update("models-key", Change{Key: id, Models: []string{"openai/gpt-5", "anthropic/*"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(Path())
	if !strings.Contains(string(b), `"models": [`) {
		t.Fatal("caller-keys.json", string(b))
	}
	secret, err = Update("rotate-key", Change{Key: id})
	if err != nil {
		t.Fatal(err)
	}
	who, ok := Authenticate(secret)
	if !ok || !slices.Equal(who.Models, []string{"openai/gpt-5", "anthropic/*"}) || !who.Allows("anthropic/claude") || who.Allows("openai/gpt-4o") {
		t.Fatal("authenticated as", who, ok)
	}
	if _, err := Update("models-key", Change{Key: id}); err != nil {
		t.Fatal(err)
	}
	if who, _ := Authenticate(secret); who.Restricted() {
		t.Fatal("every model kept a list", who)
	}
	if b, _ := os.ReadFile(Path()); strings.Contains(string(b), `"models"`) {
		t.Fatal("an empty list written", string(b))
	}
}
