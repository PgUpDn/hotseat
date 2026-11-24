package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/paths"
)

// useGOOS pins the platform layout for one test.
func useGOOS(t *testing.T, goos string) {
	old := paths.GOOS
	paths.GOOS = goos
	t.Cleanup(func() { paths.GOOS = old })
}

func TestFromEnvDefaultsStateDir(t *testing.T) {
	useGOOS(t, "darwin")
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("HOTSEAT_HOME", "")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"StateDir":          {p.StateDir, "/tmp/h/.config/hotseat"},
		"CoreDir":           {p.CoreDir(), "/tmp/h/.claude"},
		"DefaultClaudeJSON": {p.DefaultClaudeJSON(), "/tmp/h/.claude.json"},
		"RegistryFile":      {p.RegistryFile(), "/tmp/h/.config/hotseat/accounts.json"},
		"CurrentFile":       {p.CurrentFile(), "/tmp/h/.config/hotseat/current"},
		"OverridesFile":     {p.OverridesFile(), "/tmp/h/.config/hotseat/share-overrides.json"},
		"BackupsDir":        {p.BackupsDir(), "/tmp/h/.config/hotseat/backups"},
		"ShimDir":           {p.ShimDir(), "/tmp/h/.config/hotseat/bin"},
		"Zshrc":             {p.Zshrc(), "/tmp/h/.zshrc"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

func TestFromEnvHotseatHomeOverride(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("HOTSEAT_HOME", "/tmp/r")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p.StateDir != "/tmp/r" {
		t.Errorf("StateDir = %q, want /tmp/r", p.StateDir)
	}
}

func TestFromEnvRequiresHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := paths.FromEnv(); err == nil {
		t.Fatal("expected error when HOME is empty")
	}
}

func TestDesktopDataDirPerPlatform(t *testing.T) {
	p := paths.Paths{Home: "/h", StateDir: "/h/.config/hotseat"}
	useGOOS(t, "darwin")
	if got := p.DesktopDataDir(); got != "/h/Library/Application Support/Claude" {
		t.Errorf("darwin: %s", got)
	}
	useGOOS(t, "linux")
	if got := p.DesktopDataDir(); got != "/h/.config/Claude" {
		t.Errorf("linux: %s", got)
	}
}

// Linux logins usually run bash: without a ~/.zshrc the shim block and the
// aliases live in ~/.bashrc. An existing ~/.zshrc still wins.
func TestZshrcFallsBackToBashrcOnLinux(t *testing.T) {
	home := t.TempDir()
	p := paths.Paths{Home: home, StateDir: filepath.Join(home, ".config", "hotseat")}
	useGOOS(t, "linux")
	if got := p.Zshrc(); got != filepath.Join(home, ".bashrc") {
		t.Errorf("no zshrc: %s", got)
	}
	os.WriteFile(filepath.Join(home, ".zshrc"), nil, 0o644)
	if got := p.Zshrc(); got != filepath.Join(home, ".zshrc") {
		t.Errorf("with zshrc: %s", got)
	}
	useGOOS(t, "darwin")
	os.Remove(filepath.Join(home, ".zshrc"))
	if got := p.Zshrc(); got != filepath.Join(home, ".zshrc") {
		t.Errorf("darwin: %s", got)
	}
}
