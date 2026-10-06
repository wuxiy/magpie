package library

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Where a skill came from, for the page to group it with the others from
// the same repository (White Immortal on Discord: the skills of
// github.com/mattpocock/skills together, however each came in). One
// installed from GitHub says so itself; for the others it is traced:
// CC Switch's record, the skills CLI's lock (npx skills add, in
// ~/.agents/.skill-lock.json), the git checkout a linked folder is in,
// or the repository the user installed from since that has the very same
// skill (From). It only groups: such a skill is not updated from there.

// repoOf is the repository a library skill came from: owner/repo on
// GitHub, host/path elsewhere, "" when it can't be told.
func (tr *tracer) repoOf(s *Skill) string {
	if s.Source != nil && s.Source.Kind == "github" {
		return s.Source.Repo
	}
	if o, ok := ccSwitchOrigin(s); ok {
		return o.Repo
	}
	if s.From != "" {
		return s.From
	}
	dir := skillDir(s.Name)
	if !linked(dir) {
		// the library's own copy: the skills CLI's, moved in, by its name
		if e, ok := tr.lock()[s.Name]; ok && (s.Source == nil || within(realDir(s.Source.Dir), realDir(sharedSkillsDir()))) {
			return e
		}
		return ""
	}
	return tr.folder(realDir(dir))
}

// folder is the repository a skill's folder elsewhere came from: the
// skills CLI's lock for one in ~/.agents/skills, else its git checkout.
func (tr *tracer) folder(dir string) string {
	if within(dir, realDir(sharedSkillsDir())) {
		if e, ok := tr.lock()[filepath.Base(dir)]; ok {
			return e
		}
	}
	return gitRepoOf(dir)
}

// tracer reads what tracing needs once for a whole page.
type tracer struct {
	locked map[string]string
	read   bool
}

// lock is the skills CLI's lock: each skill's repository by its name.
func (tr *tracer) lock() map[string]string {
	if !tr.read {
		tr.read, tr.locked = true, readSkillLock(filepath.Join(home(), ".agents", ".skill-lock.json"))
	}
	return tr.locked
}

func readSkillLock(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f struct {
		Skills map[string]struct {
			Source     string `json:"source"`
			SourceType string `json:"sourceType"`
			SourceURL  string `json:"sourceUrl"`
		} `json:"skills"`
	}
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	out := map[string]string{}
	for name, e := range f.Skills {
		r := ""
		if e.SourceType == "github" && repoRe.MatchString(e.Source) {
			r = strings.TrimSuffix(e.Source, ".git")
		} else if e.SourceURL != "" {
			r = remoteRepo(e.SourceURL)
		}
		if r != "" {
			out[name] = r
		}
	}
	return out
}

// gitRepoOf is the repository of the git checkout a skill's folder is, or
// is in a few folders down: its remote origin. A checkout that is the home
// folder, or a folder of an app's own in it (~/.claude kept in git), is
// the user's dotfiles rather than where the skill came from.
func gitRepoOf(dir string) string {
	h := realDir(home())
	for i := 0; i < 4 && dir != "" && dir != h; i++ {
		if p := filepath.Dir(dir); p == h && strings.HasPrefix(filepath.Base(dir), ".") {
			return ""
		}
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			cfg := filepath.Join(dir, ".git", "config")
			if !fi.IsDir() {
				cfg = gitdirConfig(dir)
			}
			return originURL(cfg)
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return ""
}

// gitdirConfig is the config of a checkout whose .git is a file (a
// worktree or a submodule): the folder it names, or its common one.
func gitdirConfig(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return ""
	}
	g, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return ""
	}
	g = strings.TrimSpace(g)
	if !filepath.IsAbs(g) {
		g = filepath.Join(dir, g)
	}
	if c, err := os.ReadFile(filepath.Join(g, "commondir")); err == nil {
		cd := strings.TrimSpace(string(c))
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(g, cd)
		}
		g = cd
	}
	return filepath.Join(g, "config")
}

var originSection = regexp.MustCompile(`^\[\s*remote\s+"origin"\s*\]$`)

// originURL is the repository remote "origin" of a git config fetches.
func originURL(cfg string) string {
	b, err := os.ReadFile(cfg)
	if err != nil {
		return ""
	}
	in := false
	for _, line := range bytes.Split(b, []byte("\n")) {
		l := strings.TrimSpace(string(line))
		if strings.HasPrefix(l, "[") {
			in = originSection.MatchString(l)
			continue
		}
		if k, v, ok := strings.Cut(l, "="); in && ok && strings.TrimSpace(k) == "url" {
			return remoteRepo(strings.TrimSpace(v))
		}
	}
	return ""
}

var scpRemote = regexp.MustCompile(`^(?:[\w.-]+@)?([\w.-]+\.[\w.-]+):(.+)$`)

// remoteRepo reads a git remote's address as a repository: owner/repo on
// GitHub, host/path elsewhere. A user or a token in it is never kept.
func remoteRepo(s string) string {
	s = strings.TrimSpace(s)
	host, p := "", ""
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		host, p = u.Hostname(), u.Path
	} else if m := scpRemote.FindStringSubmatch(s); m != nil {
		host, p = m[1], m[2]
	} else if repoRe.MatchString(s) {
		host, p = "github.com", s
	} else {
		return ""
	}
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	p = strings.TrimSuffix(strings.Trim(p, "/"), ".git")
	if strings.Count(p, "/") < 1 || strings.Contains(p, "..") {
		return ""
	}
	if host == "github.com" {
		parts := strings.Split(p, "/")
		return parts[0] + "/" + parts[1]
	}
	return host + "/" + p
}

// noteFrom writes down, for the skills the library has already that a
// repository being installed from has too, that they came from there,
// when the library's SKILL.md is that repository's very own: installing
// from it again groups them with the rest of it. root is where the
// repository was fetched to.
func noteFrom(src Source, root string, cands []Candidate) {
	if src.Kind != "github" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	l, err := load()
	if err != nil {
		return
	}
	changed := false
	for _, c := range cands {
		s := l.skill(c.Name)
		if s == nil || (s.Source != nil && s.Source.Kind == "github") || s.From == src.Repo {
			continue
		}
		mine, err1 := os.ReadFile(filepath.Join(skillDir(s.Name), "SKILL.md"))
		theirs, err2 := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.Path), "SKILL.md"))
		if err1 != nil || err2 != nil || !bytes.Equal(bytes.TrimSpace(mine), bytes.TrimSpace(theirs)) {
			continue
		}
		s.From, changed = src.Repo, true
	}
	if changed {
		l.save()
	}
}
