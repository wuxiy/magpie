package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// John (Discord #feedback): a GLM Coding Plan's models read glm-5-turbo
// where ZCode's read GLM-5-Turbo. The plan's list gives ids alone, and
// Zhipu's catalog (zhipuai) doesn't list glm-5-turbo, so it kept its id as
// its name. A model Zhipu's catalog doesn't list is now named as the other
// providers serving it name it; the id sent upstream stays, a name the
// user gave stays theirs, and an Azure deployment keeps the name its owner
// gave it.
func TestModelNamedAsElsewhere(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), []byte(`{
	  "zhipuai": {"models": {"glm-5.3": {"id":"glm-5.3","name":"GLM-5.3"}}},
	  "zai": {"models": {"glm-5-turbo": {"id":"glm-5-turbo","name":"GLM-5-Turbo"}, "gpt-x": {"id":"gpt-x","name":"GPT-X"}}},
	  "zai-coding-plan": {"models": {"glm-5.3-flashx": {"id":"glm-5.3-flashx","name":"GLM-5.3-FlashX"}}}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	// as https://open.bigmodel.cn/api/coding/paas/v4/models lists them
	listed := []catalog.Model{{ID: "glm-5.3", Name: "glm-5.3"}, {ID: "glm-5-turbo", Name: "glm-5-turbo"}, {ID: "glm-5.3-flashx", Name: "glm-5.3-flashx"}, {ID: "glm-9", Name: "glm-9"}}
	if err := catalog.SaveLive("zhipu", "", listed); err != nil {
		t.Fatal(err)
	}
	if err := Save(Provider{ID: "zhipu", Name: "Zhipu GLM", Preset: "zhipu", Catalog: "zhipuai", Key: "test-key", Chat: "https://open.bigmodel.cn/api/coding/paas/v4",
		Models: []string{"glm-5.3", "glm-5-turbo", "glm-5.3-flashx", "glm-9", "gpt-x"}}); err != nil {
		t.Fatal(err)
	}
	p, err := Find("zhipu")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range p.Exposed() {
		got = append(got, m.ID+"|"+m.Name)
	}
	want := []string{
		"glm-5.3|GLM-5.3",
		"glm-5-turbo|GLM-5-Turbo",
		"glm-5.3-flashx|GLM-5.3-FlashX",
		"glm-9|glm-9", // no provider names it
		"gpt-x|GPT-X", // picked, not in the list
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if err := SetModelName("zhipu/glm-5-turbo", "turbo mine"); err != nil {
		t.Fatal(err)
	}
	byID := map[string]Entry{}
	for _, e := range Catalog() {
		byID[e.ID] = e
	}
	if e := byID["zhipu/glm-5-turbo"]; e.Name != "turbo mine" || e.Default != "GLM-5-Turbo" || e.Model != "glm-5-turbo" {
		t.Errorf("named by the user: %+v", e)
	}

	if err := catalog.SaveLive("my-azure", "", []catalog.Model{{ID: "gpt-x", Name: "gpt-x"}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(Provider{ID: "my-azure", Name: "Azure", Preset: AzurePreset, Key: "test-key", Chat: "https://me.openai.azure.com/openai/v1", Models: []string{"gpt-x"}}); err != nil {
		t.Fatal(err)
	}
	a, err := Find("my-azure")
	if err != nil {
		t.Fatal(err)
	}
	if ms := a.Exposed(); len(ms) != 1 || ms[0].Name != "gpt-x" {
		t.Errorf("an Azure deployment was renamed: %+v", ms)
	}
}
