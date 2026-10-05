package agent

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/edit"
)

func TestAsideDisconnectPreviewLeavesLiveConfigUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Aside CLI is a shell script")
	}
	settings, models := asideHome(t)
	a := mustFindAside(t)
	for _, key := range []string{"model", "fast", "image"} {
		if err := a.Apply(key, magpieID+"/relay/glm-4.6"); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{settings, models, appliedPath(), stashPath()}
	before := map[string]string{}
	for _, path := range paths {
		before[path] = readFile(path)
	}
	bin := t.TempDir()
	called := filepath.Join(bin, "daemon-called")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ASIDE_PREVIEW_CALLS\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "aside"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ASIDE_PREVIEW_CALLS", called)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		changes, err := DisconnectPreview(a, self)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, change := range changes {
			if strings.HasSuffix(change.Path, "/settings.json") {
				for _, line := range change.Lines {
					if strings.Contains(line.Text, "native-model") {
						found = true
					}
				}
			}
		}
		if !found {
			t.Errorf("preview omitted the restored native model: %+v", changes)
		}
		for _, path := range paths {
			if got := readFile(path); got != before[path] {
				t.Errorf("preview changed the live file %s", path)
			}
		}
	}
	if _, err := os.Stat(called); !os.IsNotExist(err) {
		t.Errorf("preview launched the live Aside CLI: %s", readFile(called))
	}
}

func TestAsideDisconnectDryRunNeverCallsDaemon(t *testing.T) {
	settings, models := asideHome(t)
	a := mustFindAside(t)
	if err := a.Apply("model", magpieID+"/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply("image", magpieID+"/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply("fast", magpieID+"/relay/glm-4.6"); err != nil {
		t.Fatal(err)
	}
	held := filepath.Join(t.TempDir(), dryProvidersFile)
	if err := os.WriteFile(held, []byte("relay\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(dryProvidersVar, held)
	t.Cleanup(func() {
		dryProviders = nil
		disconnectDryRun = false
	})
	calls := 0
	asideSet = func(account, expr string) error {
		calls++
		return errors.New("a preview must not reach the live daemon")
	}
	if err := DryRun("aside"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("disconnect preview called the live Aside daemon %d times", calls)
	}
	if got := asideModel(settings, "defaultModel"); got != "native/native-model" {
		t.Errorf("preview default = %q, want native/native-model", got)
	}
	if got := asideModel(settings, "modelCategories.fast"); got != "" {
		t.Errorf("preview kept a magpie role: %q", got)
	}
	if got := asideModel(settings, asideImageKey); got != "" {
		t.Errorf("preview kept a magpie image model: %q", got)
	}
	if _, ok := edit.GetJSON(models, "providers."+magpieID); ok {
		t.Error("preview kept magpie's provider")
	}
}
