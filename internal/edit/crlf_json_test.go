package edit

import "testing"

// A JSON file that uses \r\n for every line break keeps it when a member is
// added or replaced: the lines an edit adds used to carry a bare \n, so the
// file ended up with mixed endings.

func TestSetJSONKeepsCRLF(t *testing.T) {
	p := tmpFile(t, "settings.json", "{\r\n  \"a\": 1,\r\n  \"n\": {\r\n    \"x\": 1\r\n  }\r\n}\r\n")
	if err := SetJSON(p, KV{Path: "b", Value: 2}, KV{Path: "n.y", Value: []string{"p"}}); err != nil {
		t.Fatal(err)
	}
	want := "{\r\n  \"b\": 2,\r\n  \"a\": 1,\r\n  \"n\": {\r\n    \"y\": [\r\n      \"p\"\r\n    ],\r\n    \"x\": 1\r\n  }\r\n}\r\n"
	if got := read(t, p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSetJSONKeepsMixedEndings(t *testing.T) {
	p := tmpFile(t, "settings.json", "{\r\n  \"a\": 1\n}\n")
	if err := SetJSON(p, KV{Path: "b", Value: 2}); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "{\n  \"b\": 2,\r\n  \"a\": 1\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The top-level array edits do the same: VS Code on Windows writes
// chatLanguageModels.json with \r\n, and an element added or replaced there
// used to bring bare \n lines into it.
func TestSetJSONItemKeepsCRLF(t *testing.T) {
	p := tmpFile(t, "models.json", "[\r\n\t{\r\n\t\t\"name\": \"other\"\r\n\t}\r\n]\r\n")
	if err := SetJSONItem(p, map[string]string{"name": "magpie"}, map[string]any{"name": "magpie", "n": 1}); err != nil {
		t.Fatal(err)
	}
	want := "[\r\n\t{\r\n\t\t\"name\": \"other\"\r\n\t},\r\n\t{\r\n\t\t\"n\": 1,\r\n\t\t\"name\": \"magpie\"\r\n\t}\r\n]\r\n"
	if got := read(t, p); got != want {
		t.Fatalf("added: got %q, want %q", got, want)
	}
	if err := SetJSONItem(p, map[string]string{"name": "magpie"}, map[string]any{"name": "magpie", "n": 2}); err != nil {
		t.Fatal(err)
	}
	want = "[\r\n\t{\r\n\t\t\"name\": \"other\"\r\n\t},\r\n\t{\r\n\t\t\"n\": 2,\r\n\t\t\"name\": \"magpie\"\r\n\t}\r\n]\r\n"
	if got := read(t, p); got != want {
		t.Fatalf("replaced: got %q, want %q", got, want)
	}
}

func TestSetJSONItemIntoEmptyCRLFArray(t *testing.T) {
	p := tmpFile(t, "models.json", "[\r\n]\r\n")
	if err := SetJSONItem(p, map[string]string{"name": "magpie"}, map[string]any{"name": "magpie"}); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, p), "[\r\n\t{\r\n\t\t\"name\": \"magpie\"\r\n\t}\r\n]\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
