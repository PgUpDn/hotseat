package handoff

// Linux counterpart of OSA: zenity for the question, notify-send for
// notices, gnome-terminal for the resumed CLI session. (No _linux suffix:
// it must build, and be tested, on a Mac.)

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

// Default is the runner for the platform Hotseat runs on.
func Default() Runner {
	if paths.GOOS == "linux" {
		return Linux{}
	}
	return OSA{}
}

// Linux drives the desktop session of a Linux workstation.
type Linux struct{}

// dialogTimeout matches the Mac dialog's "giving up after 600".
const dialogTimeout = 600

// ZenityArgs builds the question dialog. The default button is OK, the
// first other button Cancel, any more are extra buttons. --no-markup keeps
// paths with < or & literal.
func ZenityArgs(text string, buttons []string, def string) []string {
	args := []string{"--question", "--title=Hotseat", "--no-markup", "--width=480",
		"--timeout=" + strconv.Itoa(dialogTimeout), "--text=" + text, "--ok-label=" + def}
	cancel := false
	for _, b := range buttons {
		switch {
		case b == def:
		case !cancel:
			args = append(args, "--cancel-label="+b)
			cancel = true
		default:
			args = append(args, "--extra-button="+b)
		}
	}
	return args
}

// ZenityAnswer maps zenity's exit code and output back to the pressed
// button: 0 is OK, 1 is Cancel or an extra button (whose label is printed),
// 5 is the timeout.
func ZenityAnswer(code int, out string, buttons []string, def string) (string, error) {
	out = strings.TrimSpace(out)
	switch code {
	case 0:
		return def, nil
	case 5:
		return "", nil
	case 1:
		if out != "" {
			return out, nil
		}
		for _, b := range buttons {
			if b != def {
				return b, nil
			}
		}
		return "", nil
	}
	return "", errors.New("zenity exited with " + strconv.Itoa(code))
}

func (Linux) Dialog(text string, buttons []string, def string) (string, error) {
	cmd := exec.Command("zenity", ZenityArgs(text, buttons, def)...)
	// ZENITY_OK, ZENITY_CANCEL, ... remap zenity's exit codes; without them
	// "Not now" can never read as OK.
	cmd.Env = WithoutPrefix(guiEnv(), "ZENITY_")
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return ZenityAnswer(0, string(out), buttons, def)
	case errors.As(err, &exitErr):
		return ZenityAnswer(exitErr.ExitCode(), string(out), buttons, def)
	}
	return "", err
}

func (Linux) Notify(title, text string) error {
	cmd := exec.Command("notify-send", "--app-name=Hotseat", title, text)
	cmd.Env = guiEnv()
	return cmd.Run()
}

func (Linux) SwitchDesktop(p paths.Paths, reg account.Registry, to, sessionID string) error {
	_, err := desktop.Switch(p, reg, desktop.Default(), to, desktop.SwitchOptions{Resume: sessionID, Timeout: 20 * time.Second})
	return err
}

// OpenTerminal runs the resume line in a new terminal window and keeps a
// shell open after it, like Terminal.app's "do script".
func (Linux) OpenTerminal(command string) error {
	line := command + "; exec bash"
	var cmd *exec.Cmd
	if _, err := exec.LookPath("gnome-terminal"); err == nil {
		cmd = exec.Command("gnome-terminal", "--", "bash", "-lc", line)
	} else {
		cmd = exec.Command("x-terminal-emulator", "-e", "bash", "-lc", line)
	}
	cmd.Env = TerminalEnv(guiEnv())
	return cmd.Run()
}

// TerminalEnv is env for the terminal that resumes the session as another
// account. This process is a hook of the old account's session, and a
// terminal started directly (x-terminal-emulator) hands its environment to
// the new one: nothing of Claude Code's or Anthropic's may travel that way,
// least of all a token. The resume line sets CLAUDE_CONFIG_DIR itself.
func TerminalEnv(env []string) []string {
	for _, prefix := range []string{"CLAUDE", "ANTHROPIC_"} {
		env = WithoutPrefix(env, prefix)
	}
	return env
}

// WithoutPrefix is env without the variables whose name starts with prefix.
func WithoutPrefix(env []string, prefix string) []string {
	var out []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return out
}

// guiVars are what a program needs to reach the graphical session.
var guiVars = []string{"DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY", "DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR"}

// guiEnv is the current environment, plus the graphical session's variables
// from the user's systemd instance when this process has none (started from
// an SSH login, or a hook of a session that was).
func guiEnv() []string {
	env := os.Environ()
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return env
	}
	out, err := exec.Command("systemctl", "--user", "show-environment").Output()
	if err != nil {
		return env
	}
	return MergeGUIEnv(env, string(out))
}

// MergeGUIEnv adds the session variables found in `systemctl --user
// show-environment` output to env, without overriding what env has. That
// output is shell-escaped: a value with a space or a quote comes as
// $'...'. Such a value is unescaped; one that cannot be is left out.
func MergeGUIEnv(env []string, shown string) []string {
	have := map[string]bool{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		have[k] = true
	}
	sc := bufio.NewScanner(strings.NewReader(shown))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok || have[k] {
			continue
		}
		for _, name := range guiVars {
			if k != name {
				continue
			}
			if v, ok = unescapeShown(v); ok {
				env = append(env, k+"="+v)
				have[k] = true
			}
		}
	}
	return env
}

// unescapeShown undoes systemd's escaping of one value: plain text as it
// is, or $'...' with backslash escapes.
func unescapeShown(v string) (string, bool) {
	if !strings.HasPrefix(v, "$'") {
		return v, !strings.ContainsAny(v, `'"\`)
	}
	if len(v) < 3 || !strings.HasSuffix(v, "'") {
		return "", false
	}
	body := v[2 : len(v)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\'' {
			return "", false // an unescaped quote: not one $'...' word
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i == len(body) {
			return "", false
		}
		switch body[i] {
		case '\\', '\'', '"':
			b.WriteByte(body[i])
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'x':
			if i+2 >= len(body) {
				return "", false
			}
			n, err := strconv.ParseUint(body[i+1:i+3], 16, 8)
			if err != nil {
				return "", false
			}
			b.WriteByte(byte(n))
			i += 2
		default:
			return "", false
		}
	}
	return b.String(), true
}
