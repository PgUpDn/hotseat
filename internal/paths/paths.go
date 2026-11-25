// Package paths derives every file location Hotseat uses from $HOME and
// $HOTSEAT_HOME (on Linux also $XDG_CONFIG_HOME and $SHELL), so tests can
// redirect everything to temp dirs.
package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// GOOS picks the platform's file layout. A variable so tests can check the
// Linux layout on a Mac and the other way round.
var GOOS = runtime.GOOS

type Paths struct {
	Home     string
	StateDir string
}

func FromEnv() (Paths, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return Paths{}, errors.New("HOME is not set")
	}
	stateDir := os.Getenv("HOTSEAT_HOME")
	if stateDir == "" {
		stateDir = filepath.Join(home, ".config", "hotseat")
	}
	return Paths{Home: filepath.Clean(home), StateDir: filepath.Clean(stateDir)}, nil
}

// CoreDir holds the single real copy of shared work data.
func (p Paths) CoreDir() string { return filepath.Join(p.Home, ".claude") }

// DefaultClaudeJSON is where Claude Code keeps .claude.json when
// CLAUDE_CONFIG_DIR is unset.
func (p Paths) DefaultClaudeJSON() string { return filepath.Join(p.Home, ".claude.json") }

func (p Paths) RegistryFile() string  { return filepath.Join(p.StateDir, "accounts.json") }
func (p Paths) CurrentFile() string   { return filepath.Join(p.StateDir, "current") }
func (p Paths) OverridesFile() string { return filepath.Join(p.StateDir, "share-overrides.json") }
func (p Paths) BackupsDir() string    { return filepath.Join(p.StateDir, "backups") }
func (p Paths) ShimDir() string       { return filepath.Join(p.StateDir, "bin") }
func (p Paths) UsageDir() string      { return filepath.Join(p.StateDir, "usage") }
func (p Paths) ChainFile() string     { return filepath.Join(p.StateDir, "statusline-chain.json") }

// Desktop data dirs of inactive accounts are parked under DesktopStoreDir.
func (p Paths) DesktopStoreDir() string { return filepath.Join(p.StateDir, "desktop") }
func (p Paths) DesktopProfileDir(name string) string {
	return filepath.Join(p.DesktopStoreDir(), "profiles", name)
}
func (p Paths) DesktopStateFile() string { return filepath.Join(p.DesktopStoreDir(), "state.json") }
func (p Paths) DesktopJournal() string   { return filepath.Join(p.DesktopStoreDir(), "swap.json") }

// Zshrc is the shell startup file that carries the shim's PATH block and
// the CLAUDE_CONFIG_DIR aliases: ~/.zshrc on macOS. On Linux it follows the
// login shell ($SHELL), ~/.zshrc for zsh and ~/.bashrc otherwise; both read
// the same export/alias lines. Which files happen to exist does not decide
// it: a bash user may have a stale ~/.zshrc, and creating one later must
// not orphan the block in ~/.bashrc.
func (p Paths) Zshrc() string {
	if GOOS == "linux" && filepath.Base(os.Getenv("SHELL")) != "zsh" {
		return filepath.Join(p.Home, ".bashrc")
	}
	return filepath.Join(p.Home, ".zshrc")
}

// DesktopDataDir is the default Claude Desktop data dir, Electron's
// userData: ~/Library/Application Support/Claude on macOS. On Linux it is
// Claude under $XDG_CONFIG_HOME when that is set to an absolute path (the
// XDG rule Electron follows), else ~/.config/Claude.
func (p Paths) DesktopDataDir() string {
	if GOOS == "linux" {
		if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
			return filepath.Join(filepath.Clean(dir), "Claude")
		}
		return filepath.Join(p.Home, ".config", "Claude")
	}
	return filepath.Join(p.Home, "Library", "Application Support", "Claude")
}
