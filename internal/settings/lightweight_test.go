package settings

import "testing"

// Lightweight mode (#580) is saved and read back, off by default, and is
// this computer's own: a sync or a restored backup keeps it as it is here.
func TestLightweightKeptAndOwn(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if Load().Lightweight {
		t.Fatal("on by default")
	}
	s := Load()
	s.Lightweight = true
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	if !Load().Lightweight {
		t.Fatal("not kept")
	}
	from := Settings{Theme: "dark"}
	from.KeepOwn(Load())
	if !from.Lightweight || from.Theme != "dark" {
		t.Fatalf("KeepOwn: %+v", from)
	}
	from = Settings{Lightweight: true}
	from.KeepOwn(Settings{})
	if from.Lightweight {
		t.Fatal("another computer's turned on here")
	}
}

// Keep awake (xiao_wang24004 on X) is off by default, saved, and this
// computer's own.
func TestKeepAwakeKeptAndOwn(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if Load().KeepAwake {
		t.Fatal("on by default")
	}
	s := Load()
	s.KeepAwake = true
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	if !Load().KeepAwake {
		t.Fatal("not kept")
	}
	from := Settings{}
	from.KeepOwn(Load())
	if !from.KeepAwake {
		t.Fatal("KeepOwn dropped it")
	}
	from = Settings{KeepAwake: true}
	from.KeepOwn(Settings{})
	if from.KeepAwake {
		t.Fatal("another computer's turned on here")
	}
}
