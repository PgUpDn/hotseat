package desktop_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

func useGOOS(t *testing.T, goos string) {
	t.Setenv("XDG_CONFIG_HOME", "")
	old := paths.GOOS
	paths.GOOS = goos
	t.Cleanup(func() { paths.GOOS = old })
}

// fakeProc writes /proc/<pid>/{exe,cwd,cmdline} as Linux shows them.
func fakeProc(t *testing.T, root, pid, exe, cwd string, args ...string) {
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Symlink(exe, filepath.Join(dir, "exe"))
	if cwd != "" {
		os.Symlink(cwd, filepath.Join(dir, "cwd"))
	}
	cmd := exe
	for _, a := range args {
		cmd += "\x00" + a
	}
	os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmd+"\x00"), 0o644)
}

const linuxHome = "/home/u"

func linuxProcTree(t *testing.T) string {
	root := t.TempDir()
	app := "/usr/lib/claude-desktop/claude-desktop"
	fakeProc(t, root, "100", app, linuxHome)
	fakeProc(t, root, "101", app, "/", "--type=zygote")
	fakeProc(t, root, "102", app, "/", "--type=renderer", "--lang=en-US")
	fakeProc(t, root, "103", "/usr/lib/claude-desktop/resources/disclaimer", linuxHome, "--pgroup")
	fakeProc(t, root, "104", linuxHome+"/.config/Claude/claude-code/2.1.330/abc/claude", linuxHome+"/work")
	fakeProc(t, root, "105", "/usr/lib/chatgpt/ChatGPT", linuxHome)
	fakeProc(t, root, "106", "/usr/bin/bash", linuxHome+"/.config/Claude/git-worktrees")
	fakeProc(t, root, "107", "/usr/local/bin/claude", linuxHome+"/work") // terminal claude
	fakeProc(t, root, "108", "/usr/bin/vim (deleted)", linuxHome)
	os.MkdirAll(filepath.Join(root, "self"), 0o755)
	return root
}

func pids(procs []desktop.Proc) []int {
	var out []int
	for _, p := range procs {
		out = append(out, p.PID)
	}
	sort.Ints(out)
	return out
}

func TestLinuxProcsReadsProc(t *testing.T) {
	d := desktop.LinuxDriver{Proc: linuxProcTree(t)}
	procs, err := d.Procs()
	if err != nil {
		t.Fatal(err)
	}
	if got := pids(procs); !reflect.DeepEqual(got, []int{100, 101, 102, 103, 104, 105, 106, 107, 108}) {
		t.Fatalf("pids %v", got)
	}
	for _, p := range procs {
		if p.PID == 108 && p.Exe != "/usr/bin/vim" {
			t.Errorf("deleted suffix kept: %q", p.Exe)
		}
		if p.PID == 104 && !reflect.DeepEqual(p.Paths, []string{linuxHome + "/.config/Claude/claude-code/2.1.330/abc/claude", linuxHome + "/work"}) {
			t.Errorf("paths %v", p.Paths)
		}
	}
}

// Running on Linux: the app's processes and anything run from, or working
// in, ~/.config/Claude; not other Electron apps or a terminal claude.
func TestLinuxRunning(t *testing.T) {
	useGOOS(t, "linux")
	procs, err := desktop.LinuxDriver{Proc: linuxProcTree(t)}.Procs()
	if err != nil {
		t.Fatal(err)
	}
	if got := pids(desktop.Running(procs, linuxHome)); !reflect.DeepEqual(got, []int{100, 101, 102, 103, 104, 106}) {
		t.Errorf("running %v", got)
	}
}

func TestLinuxQuitTargetsOnlyTheMainProcess(t *testing.T) {
	useGOOS(t, "linux")
	got, err := desktop.LinuxDriver{Proc: linuxProcTree(t)}.MainPIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{100}) {
		t.Errorf("main pids %v", got)
	}
}

func TestDefaultDriverPerPlatform(t *testing.T) {
	useGOOS(t, "linux")
	if _, ok := desktop.Default().(desktop.LinuxDriver); !ok {
		t.Error("linux: not LinuxDriver")
	}
	useGOOS(t, "darwin")
	if _, ok := desktop.Default().(desktop.MacDriver); !ok {
		t.Error("darwin: not MacDriver")
	}
}

func TestLinuxQuitWithoutMainProcessIsAnError(t *testing.T) {
	useGOOS(t, "linux")
	root := t.TempDir()
	fakeProc(t, root, "101", "/usr/lib/claude-desktop/claude-desktop", "/", "--type=zygote")
	if err := (desktop.LinuxDriver{Proc: root}).Quit(); !errors.Is(err, desktop.ErrNoMainProcess) {
		t.Errorf("err %v", err)
	}
}

// --wait makes a failing launcher visible; a $ would be expanded by
// systemd, so it never gets there.
func TestSessionRunArgs(t *testing.T) {
	got, err := desktop.SessionRunArgs("xdg-open", "claude://resume?session=11111111-2222-4333-8444-555555555555")
	want := []string{"systemd-run", "--user", "--quiet", "--collect", "--wait", "-p", "KillMode=process",
		"xdg-open", "claude://resume?session=11111111-2222-4333-8444-555555555555"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %q %v", got, err)
	}
	if _, err := desktop.SessionRunArgs("xdg-open", "claude://resume?session=${HOME}"); err == nil {
		t.Error("an argument with $ must be refused")
	}
}

// The Chrome extension's native host is not Desktop: on macOS exactly the
// bundled Helpers one, as before.
func TestRunningNativeHostPerPlatform(t *testing.T) {
	other := []desktop.Proc{{PID: 1, Exe: "/Applications/Claude.app/Contents/Resources/chrome-native-host"}}
	if got := desktop.Running(other, home); len(got) != 1 {
		t.Errorf("darwin: only Helpers/chrome-native-host is excluded, got %v", got)
	}
	useGOOS(t, "linux")
	host := []desktop.Proc{{PID: 1, Exe: "/usr/lib/claude-desktop/resources/chrome-native-host"}}
	if got := desktop.Running(host, linuxHome); len(got) != 0 {
		t.Errorf("linux: got %v", got)
	}
}
