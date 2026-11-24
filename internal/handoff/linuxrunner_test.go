package handoff_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/handoff"
	"github.com/zahT3/hotseat/internal/paths"
)

func TestZenityArgsDefaultIsOK(t *testing.T) {
	got := handoff.ZenityArgs("a < b & c", []string{"Not now", "Continue as work"}, "Continue as work")
	want := []string{"--question", "--title=Hotseat", "--no-markup", "--width=480", "--timeout=600",
		"--text=a < b & c", "--ok-label=Continue as work", "--cancel-label=Not now"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	extra := handoff.ZenityArgs("t", []string{"A", "B", "C"}, "B")
	if !strings.Contains(strings.Join(extra, "|"), "--cancel-label=A|--extra-button=C") {
		t.Errorf("extra %q", extra)
	}
}

func TestZenityAnswer(t *testing.T) {
	buttons := []string{"Not now", "Continue as work"}
	def := "Continue as work"
	cases := []struct {
		code int
		out  string
		want string
	}{
		{0, "", def},
		{1, "", "Not now"},
		{1, "Extra\n", "Extra"},
		{5, "", ""},
	}
	for _, c := range cases {
		got, err := handoff.ZenityAnswer(c.code, c.out, buttons, def)
		if err != nil || got != c.want {
			t.Errorf("code %d out %q: got %q %v, want %q", c.code, c.out, got, err, c.want)
		}
	}
	if _, err := handoff.ZenityAnswer(255, "", buttons, def); err == nil {
		t.Error("exit 255 should be an error")
	}
}

// From SSH or a detached hook there is no DISPLAY; the session's variables
// come from the user's systemd instance, without overriding our own.
func TestMergeGUIEnv(t *testing.T) {
	env := []string{"HOME=/home/u", "XDG_RUNTIME_DIR=/run/user/1001"}
	shown := "HOME=/home/other\nDISPLAY=:0\nXAUTHORITY=/run/user/1001/gdm/Xauthority\n" +
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus\nXDG_RUNTIME_DIR=/elsewhere\nLANG=en_US.UTF-8\n"
	got := handoff.MergeGUIEnv(env, shown)
	want := []string{"HOME=/home/u", "XDG_RUNTIME_DIR=/run/user/1001", "DISPLAY=:0",
		"XAUTHORITY=/run/user/1001/gdm/Xauthority", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestDefaultRunnerPerPlatform(t *testing.T) {
	old := paths.GOOS
	t.Cleanup(func() { paths.GOOS = old })
	paths.GOOS = "linux"
	if _, ok := handoff.Default().(handoff.Linux); !ok {
		t.Error("linux: not the Linux runner")
	}
	paths.GOOS = "darwin"
	if _, ok := handoff.Default().(handoff.OSA); !ok {
		t.Error("darwin: not OSA")
	}
}
