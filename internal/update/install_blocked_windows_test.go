package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The other install tests put text in for the new version, which Windows
// won't start: they are about moving files, so the try is left out.
func init() { canStart = func(string) error { return nil } }

// #894: a download Windows won't start isn't put in; the running exe
// stays where it is and the download goes.
func TestInstallBinaryRefusedStaysOut(t *testing.T) {
	was := canStart
	canStart = func(string) error { return &os.PathError{Op: "fork/exec", Err: errIntegrityPolicy} }
	t.Cleanup(func() { canStart = was })
	exe, staged := rollbackFiles(t)
	old, err := InstallBinaryAside(staged, exe)
	var be *BlockedError
	if !errors.As(err, &be) || !be.Policy() || old != "" {
		t.Fatalf("InstallBinaryAside = %q, %v", old, err)
	}
	wantInstallFile(t, exe, "old version")
	for _, p := range []string{staged, exe + ".old"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s: %v", filepath.Base(p), err)
		}
	}
}

// trialStart starts a real program, under the .new name a download has,
// without running it, and fails on what isn't one.
func TestTrialStart(t *testing.T) {
	ping := filepath.Join(os.Getenv("SystemRoot"), "System32", "PING.EXE")
	b, err := os.ReadFile(ping)
	if err != nil {
		t.Skip("no PING.EXE:", err)
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "magpie.exe.new")
	os.WriteFile(good, b, 0o755)
	if err := trialStart(good); err != nil {
		t.Fatalf("a program: %v", err)
	}
	bad := filepath.Join(dir, "text.exe.new")
	os.WriteFile(bad, []byte("not a program"), 0o755)
	if err := trialStart(bad); err == nil {
		t.Fatal("text started")
	}
	// ended, not left running: its file can go
	if err := os.Remove(good); err != nil {
		t.Fatalf("still held: %v", err)
	}
}

// The restart for real: a new version that can't be started, one that
// quits at once and one that runs on without saying it is up are each
// ended, their file let go of, and the running exe put back.
func TestStartCheckedPutsBackForReal(t *testing.T) {
	ping := filepath.Join(os.Getenv("SystemRoot"), "System32", "PING.EXE")
	b, err := os.ReadFile(ping)
	if err != nil {
		t.Skip("no PING.EXE:", err)
	}
	was := startWait
	startWait = 2 * time.Second
	t.Cleanup(func() { startWait = was })
	for _, c := range []struct {
		name string
		prog []byte
		args []string
	}{
		{"not a program", []byte("not a program"), nil},
		{"quits at once", b, []string{"-n", "1", "127.0.0.1"}},
		{"never says it is up", b, []string{"-n", "60", "127.0.0.1"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			exe, old := filepath.Join(dir, "magpie.exe"), filepath.Join(dir, "magpie.exe.old")
			os.WriteFile(exe, c.prog, 0o755)
			os.WriteFile(old, []byte("running version"), 0o755)
			err := startChecked(exe, old, c.args, os.Environ())
			var be *BlockedError
			if !errors.As(err, &be) {
				t.Fatalf("err = %v", err)
			}
			t.Log(err)
			wantInstallFile(t, exe, "running version")
			if _, err := os.Stat(old); !os.IsNotExist(err) {
				t.Errorf("old left: %v", err)
			}
		})
	}
}
