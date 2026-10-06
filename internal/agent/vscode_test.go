package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
	"github.com/tidwall/jsonc"
	"github.com/yetone/magpie/internal/edit"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/usage"
)

// VS Code's chat: switched on, magpie's group joins the user's in
// chatLanguageModels.json, its token in each model's Authorization header
// and no apiKey (VS Code reads that from its secret storage only), and a new
// chat starts on the model picked; switched off, both files are as they were
func TestVSCode(t *testing.T) {
	home := syncHome(t)
	dir := filepath.Join(home, "vscode", "User")
	a := vscodeAt(dir)
	settings := `{
	// my editor
	"editor.fontSize": 14,
	"chat.defaultModel": "gpt-5",
	"chat.agent.enabled": true,
}
`
	groups := `// models I added
[
	{
		"name": "Ollama",
		"vendor": "ollama", // local
		"url": "http://localhost:11434"
	},
]
`
	writeFile(t, a.Path, settings)
	lm := filepath.Join(dir, "chatLanguageModels.json")
	writeFile(t, lm, groups)
	f := a.Field("model")
	if f.Get() != "gpt-5" || !a.Detected() || a.Wired() {
		t.Fatalf("before: %q %v", f.Get(), a.Wired())
	}
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	if f.Get() != "magpie/relay/glm-4.6" || !a.Wired() || a.Check() != "" {
		t.Fatalf("connected: %q %v %q", f.Get(), a.Wired(), a.Check())
	}
	if v, _ := edit.GetJSON(a.Path, vscodeDefault); v != "relay/glm-4.6" {
		t.Fatalf("chat.defaultModel %q\n%s", v, readFile(a.Path))
	}
	for _, keep := range []string{"// my editor", `"editor.fontSize": 14`, `"chat.agent.enabled": true`} {
		if !strings.Contains(readFile(a.Path), keep) {
			t.Fatalf("settings lost %q:\n%s", keep, readFile(a.Path))
		}
	}
	var list []map[string]any
	if err := json.Unmarshal(jsonc.ToJSON([]byte(readFile(lm))), &list); err != nil || len(list) != 2 || list[0]["vendor"] != "ollama" {
		t.Fatalf("groups: %v %v\n%s", err, list, readFile(lm))
	}
	for _, keep := range []string{"// models I added", "// local"} {
		if !strings.Contains(readFile(lm), keep) {
			t.Fatalf("groups lost %q:\n%s", keep, readFile(lm))
		}
	}
	g, _ := edit.GetJSONItem(lm, vscodeGroup)
	for k, want := range map[string]string{
		"vendor": "customendpoint", "name": "magpie", "apiType": "chat-completions",
		"models.0.id": "relay/glm-4.6", "models.0.url": gatewayV1() + "/chat/completions",
		"models.0.toolCalling": "true", "models.0.contextWindow": "204800",
		"models.0.requestHeaders.Authorization": "Bearer magpie",
	} {
		if got := gjson.Get(g, k).String(); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if gjson.Get(g, "apiKey").Exists() || gjson.Get(g, "models.0.apiKey").Exists() || gjson.Get(g, "url").Exists() {
		t.Fatalf("a key or a discovery url: %s", g)
	}
	if n := gjson.Get(g, "models.0.maxOutputTokens").Int(); n <= 0 || n > 204800/4 {
		t.Fatalf("output %d", n)
	}

	// a model added to the catalog is listed once synced
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"glm-4.6", "other"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if g, _ := edit.GetJSONItem(lm, vscodeGroup); gjson.Get(g, "models.#").Int() != 2 {
		t.Fatalf("not synced: %s", g)
	}

	// the gateway's URL changed under it: Check says so
	b := readFile(lm)
	writeFile(t, lm, strings.ReplaceAll(b, gatewayV1(), "http://127.0.0.1:9/v1"))
	if c := a.Check(); !strings.Contains(c, "chatLanguageModels.json") {
		t.Fatalf("check: %q", c)
	}
	writeFile(t, lm, b)

	// one of VS Code's own picked: magpie's models stay in its list
	if err := a.Apply("model", "auto"); err != nil {
		t.Fatal(err)
	}
	if f.Get() != "auto" || !a.Wired() {
		t.Fatalf("own: %q %v", f.Get(), a.Wired())
	}
	if err := a.Apply("model", "magpie/relay/other"); err != nil {
		t.Fatal(err)
	}
	// switched off: back to auto, the last of the user's own, group gone
	if err := a.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if v, _ := edit.GetJSON(a.Path, vscodeDefault); v != "auto" || a.Wired() {
		t.Fatalf("disconnected: %q %v", v, a.Wired())
	}
	if _, ok := edit.GetJSONItem(lm, vscodeGroup); ok {
		t.Fatalf("group left:\n%s", readFile(lm))
	}
}

// on and off again with nothing else changed: both files byte for byte
func TestVSCodeRoundTrip(t *testing.T) {
	home := syncHome(t)
	dir := filepath.Join(home, "vscode", "User")
	a := vscodeAt(dir)
	settings := "{\n\t// mine\n\t\"chat.defaultModel\": \"claude-sonnet-4.5\",\n\t\"files.autoSave\": \"afterDelay\"\n}\n"
	groups := "[\n\t{\n\t\t\"name\": \"Ollama\",\n\t\t\"vendor\": \"ollama\"\n\t}\n]\n"
	lm := filepath.Join(dir, "chatLanguageModels.json")
	writeFile(t, a.Path, settings)
	writeFile(t, lm, groups)
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := a.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if readFile(a.Path) != settings || readFile(lm) != groups {
		t.Fatalf("not as it was:\n%s\n%s", readFile(a.Path), readFile(lm))
	}

	// no files at all: made, and emptied of magpie again
	home = syncHome(t)
	dir = filepath.Join(home, "fresh", "User")
	a = vscodeAt(dir)
	lm = filepath.Join(dir, "chatLanguageModels.json")
	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	if a.Field("model").Get() != "magpie/relay/glm-4.6" || a.Check() != "" {
		t.Fatalf("fresh: %q\n%s", a.Field("model").Get(), readFile(lm))
	}
	if err := a.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if v, ok := edit.GetJSON(a.Path, vscodeDefault); ok || a.Wired() {
		t.Fatalf("fresh disconnected: %q\n%s", v, readFile(a.Path))
	}
}

// a window on one of VS Code's own profiles reads that profile's
// chatLanguageModels.json and settings.json, not the default's: magpie's
// group and default model go in each profile's too, a profile that uses the
// default's is left alone, and switched off every file is as it was
// (TJHHHH on Discord: the Agents window listed magpie, the side Chat didn't)
func TestVSCodeProfiles(t *testing.T) {
	home := syncHome(t)
	dir := filepath.Join(home, "vscode", "User")
	a := vscodeAt(dir)
	work := filepath.Join(dir, "profiles", "-5f1c2a")
	shared := filepath.Join(dir, "profiles", "3b9e")
	writeFile(t, filepath.Join(dir, "globalStorage", "storage.json"), `{
	"userDataProfiles": [
		{"location": "-5f1c2a", "name": "Work", "icon": "briefcase"},
		{"location": "3b9e", "name": "Shared", "useDefaultFlags": {"settings": true, "languageModels": true}},
		{"location": "gone", "name": "Removed"}
	]
}`)
	workSettings := "{\n\t// work\n\t\"chat.defaultModel\": \"gpt-4.1\"\n}\n"
	workGroups := "[\n\t{\n\t\t\"name\": \"Ollama\",\n\t\t\"vendor\": \"ollama\"\n\t}\n]\n"
	writeFile(t, filepath.Join(work, "settings.json"), workSettings)
	writeFile(t, filepath.Join(work, "chatLanguageModels.json"), workGroups)
	writeFile(t, filepath.Join(shared, "keybindings.json"), "[]\n")

	if err := a.Connect(); err != nil {
		t.Fatal(err)
	}
	wlm := filepath.Join(work, "chatLanguageModels.json")
	g, ok := edit.GetJSONItem(wlm, vscodeGroup)
	if !ok || gjson.Get(g, "models.0.id").String() != "relay/glm-4.6" || !strings.Contains(readFile(wlm), `"ollama"`) {
		t.Fatalf("work profile's models:\n%s", readFile(wlm))
	}
	if v, _ := edit.GetJSON(filepath.Join(work, "settings.json"), vscodeDefault); v != "relay/glm-4.6" {
		t.Fatalf("work profile's chat.defaultModel %q", v)
	}
	for _, f := range []string{"settings.json", "chatLanguageModels.json"} {
		if isFile(filepath.Join(shared, f)) || isFile(filepath.Join(dir, "profiles", "gone", f)) {
			t.Fatalf("wrote %s of a profile that uses the default's, or of none", f)
		}
	}

	// a profile made after: given the group when synced
	writeFile(t, filepath.Join(dir, "globalStorage", "storage.json"), `{"userDataProfiles": [{"location": "-5f1c2a", "name": "Work"}, {"location": "new1", "name": "New"}]}`)
	writeFile(t, filepath.Join(dir, "profiles", "new1", "keybindings.json"), "[]\n")
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, ok := edit.GetJSONItem(filepath.Join(dir, "profiles", "new1", "chatLanguageModels.json"), vscodeGroup); !ok {
		t.Fatal("new profile not given magpie's group")
	}

	if err := a.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if readFile(filepath.Join(work, "settings.json")) != workSettings || readFile(wlm) != workGroups {
		t.Fatalf("work profile not as it was:\n%s\n%s", readFile(filepath.Join(work, "settings.json")), readFile(wlm))
	}
	if _, ok := edit.GetJSONItem(filepath.Join(dir, "profiles", "new1", "chatLanguageModels.json"), vscodeGroup); ok {
		t.Fatal("new profile kept magpie's group")
	}
}

// its requests come with GitHubCopilotChat/<version>
func TestVSCodeUA(t *testing.T) {
	if got := usage.AgentOf("GitHubCopilotChat/0.69.0"); got != "vscode" {
		t.Fatalf("%q", got)
	}
}

// VS Code Insiders is its own row (wani on Discord): its User folder is
// "Code - Insiders" beside VS Code's "Code", connected it writes only there,
// and its models send a token of its own, since its chat's User-Agent is
// Stable's; VS Code's files are left alone
func TestVSCodeInsiders(t *testing.T) {
	home := syncHome(t)
	ins, err := Find("vscode-insiders")
	if err != nil {
		t.Fatal(err)
	}
	st, err := Find("vscode")
	if err != nil {
		t.Fatal(err)
	}
	if ins.Name != "VS Code Insiders" || ins.Bin != "code-insiders" || filepath.Base(filepath.Dir(ins.Dir)) != "Code - Insiders" || filepath.Base(ins.Dir) != "User" ||
		filepath.Dir(filepath.Dir(ins.Dir)) != filepath.Dir(filepath.Dir(st.Dir)) || ins.UA != nil {
		t.Fatalf("insiders: %+v\nstable dir %s", ins, st.Dir)
	}
	if !strings.HasPrefix(ins.Dir, home) {
		t.Fatalf("insiders dir %s outside %s", ins.Dir, home)
	}
	for _, n := range []string{"code-insiders", "vs-code-insiders"} {
		if a, err := Find(n); err != nil || a.ID != "vscode-insiders" {
			t.Fatalf("Find(%q): %v %v", n, a, err)
		}
	}
	writeFile(t, ins.Path, "{}\n")
	writeFile(t, st.Path, `{"chat.defaultModel": "gpt-5"}`+"\n")
	if !ins.Detected() {
		t.Fatal("not detected")
	}
	if err := ins.Connect(); err != nil {
		t.Fatal(err)
	}
	g, ok := edit.GetJSONItem(filepath.Join(ins.Dir, "chatLanguageModels.json"), vscodeGroup)
	if !ok || gjson.Get(g, "models.0.requestHeaders.Authorization").String() != "Bearer magpie-vscode-insiders" || !ins.Wired() || ins.Check() != "" {
		t.Fatalf("connected: %s %v %q", g, ins.Wired(), ins.Check())
	}
	if readFile(st.Path) != `{"chat.defaultModel": "gpt-5"}`+"\n" || isFile(filepath.Join(st.Dir, "chatLanguageModels.json")) || st.Wired() {
		t.Fatalf("VS Code touched:\n%s", readFile(st.Path))
	}
	// its requests are its own, Stable's still Stable's
	if got := usage.AgentOf("vscode-insiders"); got != "vscode-insiders" {
		t.Fatalf("token: %q", got)
	}
	if got := usage.AgentOf("GitHubCopilotChat/0.69.0"); got != "vscode" {
		t.Fatalf("UA: %q", got)
	}
	if err := ins.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if _, ok := edit.GetJSONItem(filepath.Join(ins.Dir, "chatLanguageModels.json"), vscodeGroup); ok || ins.Wired() {
		t.Fatal("disconnected, magpie's group kept")
	}
}

func TestVSCodium(t *testing.T) {
	syncHome(t)
	c, err := Find("vscodium")
	if err != nil {
		t.Fatal(err)
	}
	vs, err := Find("vscode")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "VSCodium" || c.Bin != "codium" || c.UA != nil || !strings.Contains(c.Dir, filepath.Join("VSCodium", "User")) {
		t.Fatalf("vscodium: %+v", c)
	}
	for _, alias := range []string{"codium", "vscodium-chat"} {
		a, err := Find(alias)
		if err != nil || a.ID != "vscodium" {
			t.Fatalf("Find(%q): %v %v", alias, a, err)
		}
	}
	writeFile(t, c.Path, "{}\n")
	writeFile(t, vs.Path, `{"chat.defaultModel":"gpt-5"}`+"\n")
	if !c.Detected() {
		t.Fatal("VSCodium not detected")
	}
	if err := c.Connect(); err != nil {
		t.Fatal(err)
	}
	g, ok := edit.GetJSONItem(filepath.Join(c.Dir, "chatLanguageModels.json"), vscodeGroup)
	if !ok || gjson.Get(g, "models.0.requestHeaders.Authorization").String() != "Bearer magpie-vscodium" || !c.Wired() || c.Check() != "" {
		t.Fatalf("connected: %s %v %q", g, c.Wired(), c.Check())
	}
	if isFile(filepath.Join(vs.Dir, "chatLanguageModels.json")) || readFile(vs.Path) != `{"chat.defaultModel":"gpt-5"}`+"\n" || vs.Wired() {
		t.Fatalf("VS Code touched:\n%s", readFile(vs.Path))
	}
	if got := usage.AgentOf("vscodium"); got != "vscodium" {
		t.Fatalf("token: %q", got)
	}
	if err := c.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if _, ok := edit.GetJSONItem(filepath.Join(c.Dir, "chatLanguageModels.json"), vscodeGroup); ok || c.Wired() {
		t.Fatal("disconnected, magpie's group kept")
	}
}

func TestVSCodiumModelsUseTheirOwnVisibility(t *testing.T) {
	syncHome(t)
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"visible", "hidden"}}); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetHiddenModels("vscodium", []string{"relay/hidden"}); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetHiddenModels("vscode", []string{"relay/visible"}); err != nil {
		t.Fatal(err)
	}
	has := func(k vscodeKind, id string) bool {
		b, err := json.Marshal(vscodeGroupJSON(k))
		if err != nil {
			t.Fatal(err)
		}
		return gjson.GetBytes(b, "models.#(id==\""+id+"\")").Exists()
	}
	if has(vscodiumKind, "relay/hidden") || !has(vscodiumKind, "relay/visible") {
		t.Fatal("VSCodium did not use its own hidden-model list")
	}
	if !has(vscodeStable, "relay/hidden") || has(vscodeStable, "relay/visible") {
		t.Fatal("VS Code did not retain its own hidden-model list")
	}
}
