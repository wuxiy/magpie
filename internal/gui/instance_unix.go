//go:build !windows

package gui

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// heldInstance is the open lock file of the magpie that holds its
// instance; it stays open, so locked, until this process ends.
var heldInstance *os.File

// instanceLock is where the magpie running exe on the config in dir holds
// its lock. A copy elsewhere (the owner's magpie-dev) has its own.
func instanceLock(dir, exe string) string {
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	sum := sha256.Sum256([]byte(dir + "\x00" + exe))
	return filepath.Join(dir, "app-"+hex.EncodeToString(sum[:6])+".lock")
}

// holdInstance takes the lock at path for this process, or reports that
// another magpie holds it. It is a flock, which the system lets go when
// its holder ends however it ends, so a Mac shut down with magpie running
// leaves nothing behind that looks held (Crispin on Discord).
func holdInstance(path string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		f.Close()
		return false, nil
	}
	if err != nil {
		f.Close()
		return false, err
	}
	heldInstance = f
	return true, nil
}
