package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// desktopAccount makes a skills-plugin for one of Claude Desktop's
// accounts, under its folder (Claude or Claude-3p), its manifest holding
// what is given.
func desktopAccount(t *testing.T, dir, org, acct, manifest string) string {
	t.Helper()
	root := filepath.Join(dir, "local-agent-mode-sessions", "skills-plugin", org, acct)
	write(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{"name": "anthropic-skills"}`)
	write(t, filepath.Join(root, desktopManifest), manifest)
	return root
}

type desktopEntry struct {
	SkillID, Name, Description, CreatorType string
	SyncManaged, Enabled                    bool
	UpdatedAt                               any
}

func desktopEntries(t *testing.T, root string) map[string]desktopEntry {
	t.Helper()
	var doc struct {
		LastUpdated any
		Skills      []desktopEntry
	}
	if err := json.Unmarshal([]byte(read(t, filepath.Join(root, desktopManifest))), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]desktopEntry{}
	for _, e := range doc.Skills {
		out[e.SkillID] = e
	}
	return out
}

// #638: the library gives Claude Desktop's Cowork skills: a copy in every
// account's skills-plugin (no link: Cowork won't read a linked skill's
// other files) and an entry in its manifest, which Desktop lists from.
// Desktop's own skills and entries, and the user's picks of enabled, stay;
// a copy changed in Desktop isn't overwritten, and is set aside, not
// deleted, when the library stops giving it.
func TestClaudeDesktopSkills(t *testing.T) {
	h := sandbox(t)
	p, p3 := desktopFiles(t, h)
	write(t, p, `{}`)
	own := `{
  "lastUpdated": "2026-10-01T00:00:00.000Z",
  "skills": [
    {"skillId": "test", "name": "test", "description": "Test", "creatorType": "user", "syncManaged": false, "updatedAt": "2026-10-01T00:00:00.000Z", "enabled": true},
    {"skillId": "frontend-design", "name": "frontend-design", "description": "Anthropic's", "creatorType": "anthropic", "syncManaged": true, "updatedAt": "2026-10-01T00:00:00.000Z", "enabled": false}
  ]
}
`
	a := desktopAccount(t, filepath.Dir(p3), "org1", "acct1", own)
	write(t, filepath.Join(a, "skills", "test", "SKILL.md"), "---\nname: test\ndescription: Test\n---\n")
	b := desktopAccount(t, filepath.Dir(p), "org2", "acct2", `{"lastUpdated": 1759300000000, "skills": []}`)
	// an org folder with no account's plugin in it isn't one
	os.MkdirAll(filepath.Join(filepath.Dir(p), "local-agent-mode-sessions", "skills-plugin", "org3", "empty"), 0o755)

	tg := targetByID("claude-desktop")
	if tg == nil || len(tg.Desktop) != 2 || !slices.Contains(tg.Desktop, a) || !slices.Contains(tg.Desktop, b) {
		t.Fatalf("claude-desktop target: %+v", tg)
	}
	if id, err := Takes("claude-desktop", "skills"); err != nil || id != "claude-desktop" {
		t.Fatalf("Takes: %q %v", id, err)
	}
	// Desktop's own skills aren't ones to bring into the library
	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range v.FoundSkills {
		if slices.Contains(f.Agents, "claude-desktop") {
			t.Errorf("Desktop's own skill found: %+v", f)
		}
	}

	src := filepath.Join(h, "src", "skills")
	skill(t, filepath.Join(src, "pdf"), "pdf", "Read PDFs")
	write(t, filepath.Join(src, "pdf", "ref", "notes.txt"), "beside it")
	skill(t, filepath.Join(src, "frontend-design"), "frontend-design", "Mine")
	skill(t, filepath.Join(src, "test"), "test", "The library's test")
	r, err := InstallSkills(src, []string{"pdf", "frontend-design", "test"}, []string{"claude-desktop"})
	if err != nil {
		t.Fatal(err)
	}
	// Desktop's own frontend-design (built in) and test (made in Desktop)
	// stay theirs
	var clash []string
	for _, pr := range r.Problems {
		if pr.Agent == "claude-desktop" {
			clash = append(clash, pr.What)
		}
	}
	slices.Sort(clash)
	if !slices.Equal(clash, []string{"skill:frontend-design", "skill:test"}) {
		t.Errorf("problems: %+v", r.Problems)
	}
	if _, err := SkillAgents("frontend-design", nil); err != nil {
		t.Fatal(err)
	}
	ok(t)(SkillAgents("test", nil))
	for _, root := range []string{a, b} {
		d := filepath.Join(root, "skills", "pdf")
		fi, err := os.Lstat(d)
		if err != nil || !fi.IsDir() {
			t.Fatalf("%s: not a folder: %v", d, err)
		}
		if read(t, filepath.Join(d, "ref", "notes.txt")) != "beside it" {
			t.Error("the skill's other files weren't copied")
		}
		e, ok := desktopEntries(t, root)["pdf"]
		if !ok || e.Name != "pdf" || e.Description != "Read PDFs" || e.CreatorType != "user" || e.SyncManaged || !e.Enabled {
			t.Errorf("%s: entry %+v", root, e)
		}
	}
	if s := read(t, filepath.Join(a, "skills", "test", "SKILL.md")); !strings.Contains(s, "description: Test") {
		t.Errorf("Desktop's own test was overwritten:\n%s", s)
	}
	es := desktopEntries(t, a)
	if len(es) != 3 || es["frontend-design"].CreatorType != "anthropic" || es["frontend-design"].Enabled || es["test"].Description != "Test" {
		t.Errorf("Desktop's own entries changed: %+v", es)
	}
	if _, ok := desktopEntries(t, b)["pdf"].UpdatedAt.(float64); !ok {
		t.Errorf("a manifest of millisecond times got another kind: %+v", desktopEntries(t, b)["pdf"])
	}
	// the other keys of a manifest, and its layout, stay
	if s := read(t, filepath.Join(a, desktopManifest)); !strings.HasPrefix(s, "{\n  \"lastUpdated\"") {
		t.Errorf("manifest:\n%s", s)
	}

	// switched off in Desktop: it stays off when the skill changes, and
	// the copy is made again
	m := filepath.Join(b, desktopManifest)
	write(t, m, strings.Replace(read(t, m), `"enabled":true`, `"enabled":false,"pinned":true`, 1))
	lib := realDir(SkillPath("pdf"))
	write(t, filepath.Join(lib, "SKILL.md"), "---\nname: pdf\ndescription: Read PDFs well\n---\n")
	ok(t)(SkillAgents("pdf", []string{"claude-desktop"}))
	for _, root := range []string{a, b} {
		if s := read(t, filepath.Join(root, "skills", "pdf", "SKILL.md")); !strings.Contains(s, "Read PDFs well") {
			t.Errorf("%s: the copy wasn't made again:\n%s", root, s)
		}
		if e := desktopEntries(t, root)["pdf"]; e.Description != "Read PDFs well" {
			t.Errorf("%s: entry %+v", root, e)
		}
	}
	if desktopEntries(t, b)["pdf"].Enabled || !strings.Contains(read(t, m), `"pinned":true`) {
		t.Errorf("Desktop's own fields of the entry changed:\n%s", read(t, m))
	}

	// changed in Desktop: the library's next change doesn't go over it
	write(t, filepath.Join(a, "skills", "pdf", "SKILL.md"), "---\nname: pdf\ndescription: my own\n---\n")
	write(t, filepath.Join(lib, "SKILL.md"), "---\nname: pdf\ndescription: Read PDFs better\n---\n")
	r, err = SkillAgents("pdf", []string{"claude-desktop"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(r.Problems, func(pr Problem) bool {
		return pr.What == "skill:pdf" && strings.Contains(pr.Error, "changed in Claude Desktop")
	}) {
		t.Errorf("problems: %+v", r.Problems)
	}
	if s := read(t, filepath.Join(a, "skills", "pdf", "SKILL.md")); !strings.Contains(s, "my own") {
		t.Errorf("the changed copy was overwritten:\n%s", s)
	}
	if s := read(t, filepath.Join(b, "skills", "pdf", "SKILL.md")); !strings.Contains(s, "better") {
		t.Errorf("the other account's copy wasn't made again:\n%s", s)
	}

	// taken away: magpie's copies and entries go, the changed one into the
	// backups; Desktop's own stay
	ok(t)(SkillAgents("pdf", nil))
	for _, root := range []string{a, b} {
		if _, err := os.Lstat(filepath.Join(root, "skills", "pdf")); !os.IsNotExist(err) {
			t.Errorf("%s: the copy is still there", root)
		}
		if _, ok := desktopEntries(t, root)["pdf"]; ok {
			t.Errorf("%s: the entry is still there", root)
		}
	}
	kept, _ := filepath.Glob(filepath.Join(BackupDir(), "*", "*", "skills", "pdf", "SKILL.md"))
	if len(kept) != 1 || !strings.Contains(read(t, kept[0]), "my own") {
		t.Errorf("the changed copy wasn't kept: %v", kept)
	}
	if es := desktopEntries(t, a); len(es) != 2 || es["test"].Name != "test" {
		t.Errorf("Desktop's own entries: %+v", es)
	}
	if _, err := os.Stat(filepath.Join(a, "skills", "test", "SKILL.md")); err != nil {
		t.Error(err)
	}

	// given again after its folder was deleted by hand, the entry left:
	// magpie's own entry gets its folder back
	ok(t)(SkillAgents("pdf", []string{"claude-desktop"}))
	os.RemoveAll(filepath.Join(b, "skills", "pdf"))
	ok(t)(Sync())
	if _, err := os.Stat(filepath.Join(b, "skills", "pdf", "SKILL.md")); err != nil {
		t.Error(err)
	}
}
