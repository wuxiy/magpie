// Package steady reads and replaces files that another process may be
// replacing at that moment. On Windows a file being renamed over refuses
// to open, and one held open refuses to be renamed over, both for a
// moment ("Access is denied"): read as none, a file of accounts is no
// accounts, and the next write leaves them out. A file being renamed over
// is also, for a moment (two milliseconds seen), not there at all, to
// another process or thread: Stat and ReadThere wait that out for a file
// that was there. Elsewhere a rename is atomic and these are os.Stat,
// os.ReadFile and os.Rename.
package steady

import (
	"errors"
	"io/fs"
	"os"
	"runtime"
	"time"
)

// tries and pause bound the wait to half a second: what holds the file is
// another read or rename, done in milliseconds.
const (
	tries = 50
	pause = 10 * time.Millisecond
)

// goneTries and gonePause bound the wait on a file not there to 15ms or
// so: the moment a rename over it leaves is a couple of milliseconds, and
// a file that is really not there is waited on that long.
const (
	goneTries = 15
	gonePause = time.Millisecond
)

// windows is whether files behave as Windows' do; a test sets it.
var windows = runtime.GOOS == "windows"

// stat and readFile are os.Stat and os.ReadFile; a test stands in for them.
var (
	stat     = os.Stat
	readFile = os.ReadFile
)

func retry(f func() error) error {
	err := f()
	for i := 0; err != nil && windows && !errors.Is(err, fs.ErrNotExist) && i < tries; i++ {
		time.Sleep(pause)
		err = f()
	}
	return err
}

// retryGone is retry, waiting out a moment of the file not there too.
func retryGone(f func() error) error {
	err := retry(f)
	for i := 0; err != nil && windows && errors.Is(err, fs.ErrNotExist) && i < goneTries; i++ {
		time.Sleep(gonePause)
		err = retry(f)
	}
	return err
}

// Stat is os.Stat of a file that was there: one being replaced is waited
// for, so it isn't taken for gone. A file not there costs the wait, so a
// caller asking again remembers it gone (filememo does).
func Stat(p string) (fs.FileInfo, error) {
	var fi fs.FileInfo
	err := retryGone(func() (err error) {
		fi, err = stat(p)
		return err
	})
	return fi, err
}

// ReadThere is ReadFile of a file just found there (Stat): not there now is
// it being replaced, waited out.
func ReadThere(p string) ([]byte, error) {
	var b []byte
	err := retryGone(func() (err error) {
		b, err = readFile(p)
		return err
	})
	return b, err
}

// ReadFile is os.ReadFile, waiting out a file being replaced.
func ReadFile(p string) ([]byte, error) {
	var b []byte
	err := retry(func() (err error) {
		b, err = readFile(p)
		return err
	})
	return b, err
}

// Rename is os.Rename, waiting out a reader holding newpath open.
func Rename(oldpath, newpath string) error {
	return retry(func() error { return os.Rename(oldpath, newpath) })
}
