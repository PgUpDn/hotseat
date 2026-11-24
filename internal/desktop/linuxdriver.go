package desktop

// The Linux build of Claude Desktop keeps the same data layout as the Mac
// one, in ~/.config/Claude (Electron's userData), so parking and swapping
// work unchanged; only finding, quitting and starting the app differ.
// (No _linux suffix on this file: it must build, and be tested, on a Mac.)

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/zahT3/hotseat/internal/paths"
)

// linuxDesktopID is the desktop entry the Linux packages install
// (/usr/share/applications/com.anthropic.Claude.desktop).
const linuxDesktopID = "com.anthropic.Claude"

// Default is the driver for the platform Hotseat runs on.
func Default() Driver {
	if paths.GOOS == "linux" {
		return LinuxDriver{}
	}
	return MacDriver{}
}

// LinuxDriver drives Claude Desktop on Linux through /proc, signals and
// the user's systemd instance.
type LinuxDriver struct {
	// Proc is the procfs root; empty means /proc. Tests point it at a
	// fake tree.
	Proc string
}

func (d LinuxDriver) root() string {
	if d.Proc == "" {
		return "/proc"
	}
	return d.Proc
}

// Procs lists every process this user can inspect, with its executable
// and working directory from /proc: what lsof gives on macOS, without
// lsof's cost. Processes of other users fail readlink and are skipped.
func (d LinuxDriver) Procs() ([]Proc, error) {
	entries, err := os.ReadDir(d.root())
	if err != nil {
		return nil, err
	}
	var procs []Proc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		dir := filepath.Join(d.root(), e.Name())
		exe, err := os.Readlink(filepath.Join(dir, "exe"))
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")
		p := Proc{PID: pid, Exe: exe, Paths: []string{exe}}
		if cwd, err := os.Readlink(filepath.Join(dir, "cwd")); err == nil {
			p.Paths = append(p.Paths, cwd)
		}
		procs = append(procs, p)
	}
	return procs, nil
}

// AppProcs is Procs: reading /proc is already cheap.
func (d LinuxDriver) AppProcs() ([]Proc, error) { return d.Procs() }

// Alive sends signal 0, which only checks that the process exists.
func (LinuxDriver) Alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Quit sends SIGTERM to Desktop's main (browser) process. Electron treats
// it like the Quit menu item: windows close, Code sessions are told to
// stop, helpers exit. Like the Mac driver it never force-kills.
func (d LinuxDriver) Quit() error {
	pids, err := d.mainPIDs()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return nil
}

// mainPIDs finds Desktop's browser processes: the app's claude-desktop
// executable without a --type= argument.
func (d LinuxDriver) mainPIDs() ([]int, error) {
	procs, err := d.Procs()
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, p := range procs {
		if !inApp(p.Exe) || filepath.Base(p.Exe) != "claude-desktop" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(d.root(), strconv.Itoa(p.PID), "cmdline"))
		if err == nil && isMainProcess(b) {
			pids = append(pids, p.PID)
		}
	}
	return pids, nil
}

// isMainProcess reports whether a NUL-separated /proc cmdline belongs to
// Electron's browser process: its helpers (zygote, renderer, gpu,
// utility) all carry --type=.
func isMainProcess(cmdline []byte) bool {
	for _, arg := range strings.Split(string(cmdline), "\x00") {
		if strings.HasPrefix(arg, "--type=") {
			return false
		}
	}
	return len(cmdline) > 0
}

// OnlyApp reports whether every process left belongs to the app itself:
// no Code session (run from the data dir) is among them.
func OnlyApp(left []Proc) bool {
	for _, p := range left {
		if !inApp(p.Exe) {
			return false
		}
	}
	return len(left) > 0
}

// KillStuck ends a Desktop that finished quitting but did not exit. Seen
// on Linux: after SIGTERM Desktop runs its cleanup, its sessions end, then
// the main process spins relaunching a dying GPU process and its own
// "forcing exit" watchdog does not get it out; the windows are gone and
// the data dir stays locked. Only the app's own processes are killed, and
// only when no session process is left, so a working session is never cut.
func (LinuxDriver) KillStuck(left []Proc) bool {
	if !OnlyApp(left) {
		return false
	}
	for _, p := range left {
		_ = syscall.Kill(p.PID, syscall.SIGKILL)
	}
	return true
}

// Launch and OpenURL go through `systemd-run --user`, so the app starts
// inside the graphical session (its DISPLAY, Wayland socket and session
// bus) even when hotseat runs from an SSH login or a hook.
// KillMode=process: the launcher exits at once, and without it systemd
// would kill the app it just started along with the unit.
func (LinuxDriver) Launch() error {
	return sessionRun("gtk-launch", linuxDesktopID)
}

func (LinuxDriver) OpenURL(url string) error {
	return sessionRun("xdg-open", url)
}

func sessionRun(args ...string) error {
	return exec.Command("systemd-run", append([]string{"--user", "--quiet", "--collect", "-p", "KillMode=process"}, args...)...).Run()
}
