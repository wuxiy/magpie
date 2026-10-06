package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoIn(t *testing.T) map[string]string {
	t.Helper()
	v, err := Read(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, s := range v.Skills {
		out[s.Name] = s.Repo
	}
	return out
}

// A skill not installed from GitHub is grouped with the repository it
// came from all the same (White Immortal on Discord): the skills CLI's
// lock names it, or the git checkout it's in, whatever the remote's form,
// and never with a user or token the remote had. A skill in ~/.claude kept
// in git (the user's dotfiles) is not from that repository.
func TestSkillRepoTraced(t *testing.T) {
	h := sandbox(t)
	// npx skills add mattpocock/skills: in ~/.agents/skills, in its lock
	skill(t, filepath.Join(h, ".agents/skills/grill-me"), "grill-me", "Grill")
	write(t, filepath.Join(h, ".agents/.skill-lock.json"), `{"version":3,"skills":{
		"grill-me":{"source":"mattpocock/skills","sourceType":"github","sourceUrl":"https://github.com/mattpocock/skills.git","skillPath":"skills/grill-me/SKILL.md"},
		"tdd":{"source":"x","sourceType":"git","sourceUrl":"git@gitlab.com:team/skills.git"}}}`)
	os.MkdirAll(filepath.Join(h, ".claude/skills"), 0o755)
	dirLink(filepath.Join(h, ".agents/skills/grill-me"), filepath.Join(h, ".claude/skills/grill-me"))
	// a clone of the repository, linked from the library
	clone := filepath.Join(h, "code/skills")
	skill(t, filepath.Join(clone, "skills/write-a-prd"), "write-a-prd", "PRD")
	write(t, filepath.Join(clone, ".git/config"), "[core]\n\tbare = false\n[remote \"upstream\"]\n\turl = https://github.com/other/fork.git\n[remote \"origin\"]\r\n\turl = https://me:ghp_secret@github.com/mattpocock/skills.git\r\n")
	// a skill that is a clone of its own, moved in
	skill(t, filepath.Join(h, ".claude/skills/review"), "review", "Review")
	write(t, filepath.Join(h, ".claude/skills/review/.git/config"), "[remote \"origin\"]\n\turl = git@gitlab.com:me/review.git\n")
	// ~/.claude in git: the user's dotfiles
	write(t, filepath.Join(h, ".claude/.git/config"), "[remote \"origin\"]\n\turl = git@github.com:me/dotfiles.git\n")
	skill(t, filepath.Join(h, ".claude/skills/notes"), "notes", "Mine")

	ok(t)(ImportSkills([]string{"grill-me", "review", "notes"}))
	p, err := ProbeSkills(filepath.Join(clone, "skills/write-a-prd"))
	if err != nil || len(p.Candidates) != 1 {
		t.Fatalf("probe: %+v %v", p, err)
	}
	ok(t)(InstallSkills(filepath.Join(clone, "skills/write-a-prd"), []string{p.Candidates[0].Path}, nil))

	got := repoIn(t)
	want := map[string]string{"grill-me": "mattpocock/skills", "write-a-prd": "mattpocock/skills", "review": "gitlab.com/me/review", "notes": ""}
	for n, r := range want {
		if got[n] != r {
			t.Errorf("%s: repo %q, want %q", n, got[n], r)
		}
	}
	b, _ := os.ReadFile(path())
	if strings.Contains(string(b), "ghp_secret") {
		t.Error("the remote's token was written down")
	}
	// the clone moved in kept where it came from
	if s := mustLoad(t).skill("review"); s.From != "gitlab.com/me/review" {
		t.Errorf("review's From: %q", s.From)
	}
}

// The remote forms git takes, each read as the repository.
func TestRemoteRepo(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/mattpocock/skills":          "mattpocock/skills",
		"https://www.github.com/mattpocock/skills.git/": "mattpocock/skills",
		"git@github.com:mattpocock/skills.git":          "mattpocock/skills",
		"ssh://git@github.com/mattpocock/skills.git":    "mattpocock/skills",
		"https://user:tok@github.com/a/b/tree/main/x":   "a/b",
		"git@gitlab.com:group/sub/proj.git":             "gitlab.com/group/sub/proj",
		"https://tok@codeberg.org/me/s.git":             "codeberg.org/me/s",
		"mattpocock/skills":                             "mattpocock/skills",
		"/Users/me/skills":                              "",
		"https://github.com/onlyowner":                  "",
		"https://example.com/../etc":                    "",
	} {
		if got := remoteRepo(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

// Installing from a repository again groups the skills the library has
// already from it (White Immortal: installed before, from that very
// address): those whose SKILL.md is the repository's own. One of the same
// name that differs is someone else's skill, and stays where it was.
func TestInstallAgainNotesFrom(t *testing.T) {
	h := sandbox(t)
	newFakeGitHub(t)
	// the fake repository's skills/pdf/SKILL.md, as installed by hand
	write(t, filepath.Join(h, ".claude/skills/pdf/SKILL.md"), "---\nname: pdf\ndescription: PDFs\n---\n")
	ok(t)(ImportSkill("pdf"))
	if r := repoIn(t)["pdf"]; r != "" {
		t.Fatalf("pdf has a repo before: %q", r)
	}
	if _, err := ProbeSkills("someone/fromagain"); err != nil {
		t.Fatal(err)
	}
	if r := repoIn(t)["pdf"]; r != "someone/fromagain" {
		t.Errorf("pdf after installing from its repository again: %q", r)
	}

	// another's pdf, not the same file: not noted
	h = sandbox(t)
	write(t, filepath.Join(h, ".claude/skills/pdf/SKILL.md"), "---\nname: pdf\ndescription: My own\n---\n")
	ok(t)(ImportSkill("pdf"))
	if _, err := ProbeSkills("someone/notmine"); err != nil {
		t.Fatal(err)
	}
	if r := repoIn(t)["pdf"]; r != "" {
		t.Errorf("another's pdf grouped with the repository: %q", r)
	}
}

func mustLoad(t *testing.T) *Library {
	t.Helper()
	l, err := load()
	if err != nil {
		t.Fatal(err)
	}
	return l
}
