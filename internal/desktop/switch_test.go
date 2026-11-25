package desktop_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

// sessCS is the CLI session of the Desktop session focused last.
const sessCS = "11111111-2222-4333-8444-555555555555"

func writeConfig(t *testing.T, dir, acct string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCache":"SECRET-TOKEN","lastKnownAccountUuid":"`+acct+`"}`)
}

func focusIndex(t *testing.T, dir, acct, org, id, cli string, focused int64) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"sessionId": "local_" + id, "cliSessionId": cli, "lastFocusedAt": focused, "lastActivityAt": focused})
	testutil.WriteFile(t, filepath.Join(dir, "claude-code-sessions", acct, org, "local_"+id+".json"), string(b))
}

func switchSetup(t *testing.T) (paths.Paths, account.Registry) {
	t.Helper()
	p := testutil.FakeHome(t)
	reg := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2"), AccountUUID: "b2", OrgUUID: "o2"},
	}}
	active := p.DesktopDataDir()
	writeConfig(t, active, "a1")
	focusIndex(t, active, "a1", "o1", "S", sessCS, 10)
	focusIndex(t, active, "a1", "o1", "T", "ct", 5)
	testutil.WriteFile(t, filepath.Join(active, "claude_desktop_config.json"), `{"mcpServers":{"m":{"command":"x"}},"preferences":{}}`)
	writeConfig(t, p.DesktopProfileDir("claude2"), "b2")
	for _, id := range []string{sessCS, "ct"} {
		testutil.WriteFile(t, filepath.Join(p.CoreDir(), "projects", "-x", id+".jsonl"), "{}")
	}
	desktop.SaveState(p, desktop.State{Active: "claude", Profiles: map[string]string{"claude": "a1", "claude2": "b2"}})
	return p, reg
}

func acctOf(t *testing.T, dir string) string {
	t.Helper()
	id, _ := desktop.AccountUUID(dir)
	return id
}

func TestCurrentSession(t *testing.T) {
	p, _ := switchSetup(t)
	if id, ok := desktop.CurrentSession(p.DesktopDataDir(), "a1", "o1"); !ok || id != sessCS {
		t.Errorf("got %q %v", id, ok)
	}
}

func TestSwitchSyncsSwapsAndResumes(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{procs: claudeApp, stopsOn: 1}
	res, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Resume: "current", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if acctOf(t, p.DesktopDataDir()) != "b2" || acctOf(t, p.DesktopProfileDir("claude")) != "a1" {
		t.Errorf("real path=%s parked claude=%s", acctOf(t, p.DesktopDataDir()), acctOf(t, p.DesktopProfileDir("claude")))
	}
	for _, id := range []string{"S", "T"} {
		if _, err := os.Stat(filepath.Join(p.DesktopDataDir(), "claude-code-sessions", "b2", "o2", "local_"+id+".json")); err != nil {
			t.Errorf("session %s not in claude2's list", id)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(p.DesktopDataDir(), "claude_desktop_config.json")); !strings.Contains(string(b), `"m"`) {
		t.Errorf("MCP not synced: %s", b)
	}
	if len(d.urls) != 1 || d.urls[0] != "claude://resume?session="+sessCS || d.quits != 1 {
		t.Errorf("urls %v quits %d", d.urls, d.quits)
	}
	if s, _ := desktop.LoadState(p); s.Active != "claude2" || res.To != "claude2" {
		t.Errorf("state %+v result %+v", s, res)
	}
}

func TestSwitchRefusesWhileRunning(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{procs: claudeApp}
	_, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Timeout: 600 * time.Millisecond})
	if err == nil || acctOf(t, p.DesktopDataDir()) != "a1" || acctOf(t, p.DesktopProfileDir("claude2")) != "b2" {
		t.Errorf("err %v; real path=%s", err, acctOf(t, p.DesktopDataDir()))
	}
}

func TestSwitchRefusesIdentityMismatch(t *testing.T) {
	p, reg := switchSetup(t)
	writeConfig(t, p.DesktopDataDir(), "zz")
	_, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "zz") || acctOf(t, p.DesktopProfileDir("claude2")) != "b2" {
		t.Errorf("err %v", err)
	}
}

func TestSwitchNoopSameAccount(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{}
	if _, err := desktop.Switch(p, reg, d, "claude", desktop.SwitchOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if d.quits != 0 || len(d.urls) != 0 || d.opened != 0 {
		t.Errorf("driver used: %+v", d)
	}
}

func TestSwitchToMissingProfile(t *testing.T) {
	p, reg := switchSetup(t)
	reg.Accounts = append(reg.Accounts, account.Account{Name: "claude3", AccountUUID: "c3", OrgUUID: "o3"})
	_, err := desktop.Switch(p, reg, &fakeDriver{}, "claude3", desktop.SwitchOptions{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "desktop add claude3") {
		t.Errorf("err %v", err)
	}
}

func TestSwitchFreshProfileLearnsIdentity(t *testing.T) {
	p, reg := switchSetup(t)
	desktop.SaveState(p, desktop.State{Active: "claude", Profiles: map[string]string{"claude": "", "claude2": "b2"}})
	if _, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if s, _ := desktop.LoadState(p); s.Profiles["claude"] != "a1" {
		t.Errorf("identity of the fresh profile not recorded: %+v", s)
	}
}

// The session ID goes into a claude:// URL for the system's opener: one
// that is not a UUID is refused before Desktop is quit or anything moves.
func TestSwitchRefusesSessionIDThatIsNotAUUID(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{procs: claudeApp, stopsOn: 1}
	_, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Resume: "${HOME}", Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "not a UUID") {
		t.Fatalf("err %v", err)
	}
	if d.quits != 0 || len(d.urls) != 0 || acctOf(t, p.DesktopDataDir()) != "a1" {
		t.Errorf("quits %d urls %v active %s", d.quits, d.urls, acctOf(t, p.DesktopDataDir()))
	}
}

// "current" comes from Desktop's own files; a bad one there does not block
// the switch, Desktop just opens without a session.
func TestSwitchDropsCurrentSessionThatIsNotAUUID(t *testing.T) {
	p, reg := switchSetup(t)
	focusIndex(t, p.DesktopDataDir(), "a1", "o1", "U", "$(x)", 99)
	d := &fakeDriver{procs: claudeApp, stopsOn: 1}
	res, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Resume: "current", Timeout: 2 * time.Second})
	if err != nil || res.Resumed != "" || len(d.urls) != 0 || d.opened != 1 {
		t.Errorf("err %v resumed %q urls %v opened %d", err, res.Resumed, d.urls, d.opened)
	}
}

type noLaunchDriver struct{ fakeDriver }

func (n *noLaunchDriver) Launch() error { n.opened++; return os.ErrNotExist }

// A launcher that fails after the swap is reported as that: the swap
// stays, and nothing tries to launch a second time.
func TestSwitchReportsDesktopNotReopened(t *testing.T) {
	p, reg := switchSetup(t)
	d := &noLaunchDriver{fakeDriver{procs: claudeApp, stopsOn: 1}}
	res, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Timeout: 2 * time.Second})
	if !errors.Is(err, desktop.ErrNotReopened) || !strings.Contains(err.Error(), "now set to claude2") {
		t.Fatalf("err %v", err)
	}
	if acctOf(t, p.DesktopDataDir()) != "b2" || res.To != "claude2" || d.opened != 1 {
		t.Errorf("active %s result %+v launches %d", acctOf(t, p.DesktopDataDir()), res, d.opened)
	}
}
