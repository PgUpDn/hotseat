# Hotseat top-bar app for Linux (draft)

`hotseat-indicator` is a port of the macOS menu bar app (`macos/Sources/Hotseat`) to GTK: a top-bar icon whose menu has one card per account, then Settings…, Setup…, Refresh and Quit; a Settings window (Accounts, General); and a Setup window with the same steps and wording as the Mac app.

Like the Mac app it only displays and clicks. Everything shown comes from `hotseat panel --json` and `hotseat setup status --json`, and every action runs the `hotseat` CLI (`use`, `desktop switch`, `desktop add`, `note`, `icon`, `remove`, the setup commands). It makes no network requests and reads none of Claude's files.

## Requirements

- Python 3 with PyGObject and GTK 3 (stock on Ubuntu; `python3-gi`, `gir1.2-gtk-3.0`).
- A desktop that shows legacy tray icons. Tried on Ubuntu 24.04 (GNOME 46, X11 session), where the stock AppIndicator extension shows them. Not tried on Wayland or other desktops.
- The Linux `hotseat` CLI in `~/.local/bin/hotseat` (see the CLI's Linux support).

## Run

```sh
install -m 755 linux/hotseat-indicator ~/.local/bin/hotseat-indicator
hotseat-indicator &
```

"Launch at Login" in Settings writes `~/.config/autostart/hotseat-indicator.desktop`. To try it on made-up accounts: `HOTSEAT_PATH="$PWD/macos/demo/hotseat-demo" linux/hotseat-indicator`.

## Differences from the Mac app

All forced by the platform:

- No text beside the icon (a tray icon is a square image): the icon is the current account's initial or emoji, blue, amber from 80 %, red at a limit; the numbers are in the tooltip.
- Shortcuts are Ctrl+1…9, Ctrl+`,`, Ctrl+R and Ctrl+Q while the menu is open.
- Account icons are emoji; an SF Symbol name picked on a Mac shows as the matching emoji.
- There is no bundled CLI to install: Setup's first step only checks that `~/.local/bin/hotseat` exists.
- "Add Account…" opens `hotseat add <name>` in gnome-terminal (or `x-terminal-emulator`).

## Open questions

- It is one Python file with no tests. The text formatting follows `HotseatKit/Format.swift`; if this is wanted in the repo, those functions should get tests (and could move to a module).
- `Gtk.StatusIcon` is deprecated in GTK 3 and gone in GTK 4. AppIndicator (`gir1.2-ayatanaappindicator3-0.1`) is the supported way but cannot open a custom window on click, only a plain menu, so the cards would have to become menu rows.
