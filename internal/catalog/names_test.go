package catalog

import (
	"strings"
	"testing"
)

// A model is named as most of the providers serving it name it, matched
// without a vendor's prefix and in any case; a name that is only the id
// casts no vote.
func TestNameOf(t *testing.T) {
	writeCatalog(t, `{
	  "zai": {"models": {"glm-5-turbo": {"id":"glm-5-turbo","name":"GLM-5-Turbo"}}},
	  "zai-coding-plan": {"models": {"glm-5-turbo": {"id":"glm-5-turbo","name":"GLM-5-Turbo"}}},
	  "a": {"models": {"z-ai/glm-5-turbo": {"id":"z-ai/glm-5-turbo","name":"GLM 5 Turbo"}}},
	  "b": {"models": {"glm-5-turbo": {"id":"glm-5-turbo","name":"glm-5-turbo"}},
	        "plain": {"id":"plain","name":"plain"}}
	}`)
	for id, want := range map[string]string{
		"glm-5-turbo":      "GLM-5-Turbo",
		"GLM-5-Turbo":      "GLM-5-Turbo",
		"z-ai/glm-5-turbo": "GLM-5-Turbo",
		"plain":            "",
		"unknown":          "",
	} {
		if got := NameOf(id); got != want {
			t.Errorf("NameOf(%q) = %q, want %q", id, got, want)
		}
	}
	in := []Model{
		{ID: "glm-5-turbo", Name: "glm-5-turbo"},
		{ID: "GLM-5-Turbo"},
		{ID: "glm-5-turbo", Name: "Turbo"}, // the list's own name stays
		{ID: "plain", Name: "plain"},
	}
	var got []string
	for _, m := range Named(in) {
		got = append(got, m.ID+"|"+m.Name)
	}
	want := "glm-5-turbo|GLM-5-Turbo\nGLM-5-Turbo|GLM-5-Turbo\nglm-5-turbo|Turbo\nplain|plain"
	if strings.Join(got, "\n") != want {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), want)
	}
	if in[0].Name != "glm-5-turbo" {
		t.Fatalf("the list given was changed: %q", in[0].Name)
	}
}
