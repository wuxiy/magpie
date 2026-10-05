package proc

import (
	"os"
	"path/filepath"
	"testing"
)

// A tool off PATH is found in the folders it is installed in, by its
// Windows name too (#839: ~/.local/bin/claude.exe was looked for as
// ~/.local/bin/claude, so Windows' Claude sign-in said none was installed).
func TestToolIn(t *testing.T) {
	d := t.TempDir()
	empty, bin, npm := filepath.Join(d, "empty"), filepath.Join(d, "bin"), filepath.Join(d, "npm")
	for _, x := range []string{empty, bin, npm} {
		os.MkdirAll(x, 0o755)
	}
	os.WriteFile(filepath.Join(bin, "claude.exe"), nil, 0o755)
	os.WriteFile(filepath.Join(npm, "codex.cmd"), nil, 0o755)
	os.MkdirAll(filepath.Join(npm, "claude"), 0o755) // a folder isn't the tool
	if got := toolIn("claude", "windows", []string{"", empty, npm, bin}); got != filepath.Join(bin, "claude.exe") {
		t.Errorf("windows claude: %q", got)
	}
	if got := toolIn("codex", "windows", []string{bin, npm}); got != filepath.Join(npm, "codex.cmd") {
		t.Errorf("windows codex: %q", got)
	}
	if got := toolIn("claude", "darwin", []string{bin, npm}); got != "" {
		t.Errorf("darwin: %q", got)
	}
	os.WriteFile(filepath.Join(bin, "claude"), nil, 0o755)
	if got := toolIn("claude", "linux", []string{empty, bin}); got != filepath.Join(bin, "claude") {
		t.Errorf("linux: %q", got)
	}
}
