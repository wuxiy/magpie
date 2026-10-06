package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Valid JSON can still be unreadable as saved accounts. On a cold start,
// adding an account must keep those original credentials in a backup.
func TestUnreadLoginsFieldTypesKeptBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
	}{
		{"boolean as string", `[{"agent":"antigravity","user":"old@example.com","on":"true","auth":{"refresh_token":"old-refresh-token"}}]`},
		{"invalid time", `[{"agent":"antigravity","user":"old@example.com","seen":"yesterday","auth":{"refresh_token":"old-refresh-token"}}]`},
		{"object instead of list", `{"agent":"antigravity","user":"old@example.com","auth":{"refresh_token":"old-refresh-token"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			googleSandbox(t, &fakeGoogle{})
			if !json.Valid([]byte(tc.text)) {
				t.Fatal("fixture must be syntactically valid JSON")
			}
			p := loginsPath()
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(tc.text), 0o600); err != nil {
				t.Fatal(err)
			}
			lastLoginsMu.Lock()
			lastLogins = nil // magpie just started: no previously read accounts
			lastLoginsMu.Unlock()
			if err := addGoogleLogin("antigravity", "new@example.com", "", googleAuth{RefreshToken: "new-refresh-token", Project: "p"}); err != nil {
				t.Fatal(err)
			}
			kept, err := filepath.Glob(p + ".bad-*")
			if err != nil {
				t.Fatal(err)
			}
			if len(kept) != 1 {
				t.Fatalf("unreadable saved accounts not kept before adding an account: %v", kept)
			}
			b, err := os.ReadFile(kept[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.text {
				t.Fatalf("backup = %q, want original credentials %q", b, tc.text)
			}
			if ls := readLogins(); len(ls) != 1 || ls[0].User != "new@example.com" {
				t.Fatalf("new account not saved: %v", ls)
			}
		})
	}
}

func TestReadableLoginsNotKeptAside(t *testing.T) {
	googleSandbox(t, &fakeGoogle{})
	for _, u := range []string{"a@x.com", "b@x.com"} {
		if err := addGoogleLogin("antigravity", u, "", googleAuth{RefreshToken: "1//" + u, Project: "p"}); err != nil {
			t.Fatal(err)
		}
	}
	kept, err := filepath.Glob(loginsPath() + ".bad-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 0 {
		t.Fatalf("readable saved accounts unnecessarily backed up: %v", kept)
	}
	if ls := readLogins(); len(ls) != 2 {
		t.Fatalf("readable accounts lost when adding an account: %v", ls)
	}
}

// A logins.json that doesn't parse (half written, or written by something
// else) was read as no accounts, and the next account added wrote the
// file anew with that one alone: every saved sign-in gone.
func TestUnreadLoginsNotWrittenOver(t *testing.T) {
	googleSandbox(t, &fakeGoogle{})
	for _, u := range []string{"a@x.com", "b@x.com"} {
		if err := addGoogleLogin("antigravity", u, "", googleAuth{RefreshToken: "1//" + u, Project: "p"}); err != nil {
			t.Fatal(err)
		}
	}
	p := loginsPath()
	good, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	users := func() []string {
		var us []string
		for _, l := range readLogins() {
			us = append(us, l.User)
		}
		slices.Sort(us)
		return us
	}

	// a read that fails: the accounts read before are kept
	os.WriteFile(p, good[:len(good)/2], 0o600)
	if us := users(); strings.Join(us, ",") != "a@x.com,b@x.com" {
		t.Fatalf("a logins.json half there read as %v", us)
	}
	if err := addGoogleLogin("antigravity", "c@x.com", "", googleAuth{RefreshToken: "1//c", Project: "p"}); err != nil {
		t.Fatal(err)
	}
	if us := users(); strings.Join(us, ",") != "a@x.com,b@x.com,c@x.com" {
		t.Fatalf("after an account added: %v", us)
	}

	// nothing read before (magpie just started): the file is kept aside
	// before it is written over. The add above kept its own copy, named by
	// the second it was made, so only the copies made from here count.
	before, _ := filepath.Glob(p + ".bad-*")
	for _, f := range before {
		os.Remove(f)
	}
	half := good[:len(good)/2]
	os.WriteFile(p, half, 0o600)
	lastLoginsMu.Lock()
	lastLogins = nil
	lastLoginsMu.Unlock()
	if err := addGoogleLogin("antigravity", "d@x.com", "", googleAuth{RefreshToken: "1//d", Project: "p"}); err != nil {
		t.Fatal(err)
	}
	kept, _ := filepath.Glob(p + ".bad-*")
	if len(kept) != 1 {
		t.Fatalf("logins.json that didn't parse not kept aside: %v", kept)
	}
	if b, _ := os.ReadFile(kept[0]); string(b) != string(half) {
		t.Fatalf("kept %q", b)
	}
}
