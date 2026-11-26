"""Text and small decisions for hotseat-indicator, without GTK, so they can
be tested: the strings follow macos/Sources/HotseatKit/Format.swift, the
account helpers Snapshot.swift, the setup steps Setup.swift.

Accounts, windows and limits are the dicts of `hotseat panel --json`.
`tz` is a tzinfo for tests; None means the local time zone.
"""
import datetime as dt
import json
import os
import re

HOME = os.path.expanduser("~")

# The Mac app's SF Symbol choices and their emoji counterparts. An account
# whose icon was picked on a Mac shows the matching emoji here.
ICON_CHOICES = [
    ("person.fill", "👤"), ("briefcase.fill", "💼"), ("house.fill", "🏠"), ("star.fill", "⭐"),
    ("bolt.fill", "⚡"), ("leaf.fill", "🍃"), ("flame.fill", "🔥"), ("moon.fill", "🌙"),
    ("sun.max.fill", "☀️"), ("heart.fill", "❤️"), ("graduationcap.fill", "🎓"), ("hammer.fill", "🔨"),
    ("paintbrush.fill", "🖌️"), ("gamecontroller.fill", "🎮"), ("airplane", "✈️"), ("cup.and.saucer.fill", "☕"),
]
SYMBOL_EMOJI = dict(ICON_CHOICES)


# ---------- usage windows and limits ----------

def win_reset(w, now):
    """Whether the window has reset by now, as reported or because its reset
    time has passed since the snapshot was taken."""
    return bool(w.get("reset")) or (w.get("resetsAt", 0) > 0 and w["resetsAt"] <= now)


def win_percent(w, now):
    return 0.0 if win_reset(w, now) else float(w.get("usedPercent", 0))


def _local(unix, tz):
    return dt.datetime.fromtimestamp(unix, tz) if tz else dt.datetime.fromtimestamp(unix)


def fmt_when(unix, now, tz=None):
    t, n = _local(unix, tz), _local(now, tz)
    if t.date() == n.date():
        return t.strftime("%H:%M")
    if unix - now < 6 * 86400:
        return t.strftime("%a %H:%M")
    return "%s %d %s" % (t.strftime("%b"), t.day, t.strftime("%H:%M"))


def fmt_resets(unix, now, tz=None):
    """A countdown within a day ("in 3h 53m"), a weekday and time after."""
    left = unix - now
    if left >= 86400:
        return fmt_when(unix, now, tz)
    h, m = divmod(int(max(0, (left + 59) // 60)), 60)
    if h == 0:
        return "in %dm" % m
    return "in %dh" % h if m == 0 else "in %dh %dm" % (h, m)


def fmt_value(w, now, tz=None):
    """A usage window's number and reset time, shown beside its bar."""
    if not w:
        return "?"
    if win_reset(w, now):
        return "0% (reset)"
    s = "%.0f%%" % w.get("usedPercent", 0)
    if w.get("resetsAt", 0) > 0:
        s += " · " + fmt_resets(w["resetsAt"], now, tz)
    return s


def fmt_age(observed, now):
    if not observed or observed <= 0:
        return "no data yet"
    d = max(0, now - observed)
    if d < 60:
        return "updated just now"
    if d < 3600:
        return "updated %d min ago" % (d // 60)
    if d < 48 * 3600:
        return "updated %d h ago" % (d // 3600)
    return "updated %d d ago" % (d // 86400)


def _fmt_limit(name, limit, now, tz):
    if not limit or limit.get("resetsAt", 0) <= 0:
        return name + " reached"
    if limit["resetsAt"] - now < 86400:
        return name + " · resets " + fmt_resets(limit["resetsAt"], now, tz)
    return name + " until " + fmt_when(limit["resetsAt"], now, tz)


def fmt_limit(limit, now, tz=None):
    return _fmt_limit("Limit", limit, now, tz)


def fmt_model_limit(limit, now, tz=None):
    """A limit on one model: "Fable limit · resets in 5h 30m"."""
    return _fmt_limit((limit.get("model") or "Model") + " limit", limit, now, tz)


def is_limited(a, now):
    """Whether the account-wide limit still holds now (the snapshot may be
    old)."""
    if not a.get("limitActive"):
        return False
    r = (a.get("limit") or {}).get("resetsAt", 0)
    return r <= 0 or r > now


def is_model_limited(a, now):
    """Whether the limit on one model still holds now. It does not make the
    account limited."""
    limit = a.get("modelLimit")
    if not a.get("modelLimitActive") or not limit:
        return False
    r = limit.get("resetsAt", 0)
    return r <= 0 or r > now


def windows(a):
    return [w for w in (a.get("fiveHour"), a.get("sevenDay")) if w]


def observed_at(a):
    """When the newer of the usage windows was observed; 0 when none was."""
    return max([w.get("observedAt", 0) for w in windows(a)] or [0])


# ---------- accounts ----------

def display_name(a):
    """The user's note, else the name part of the email, else Hotseat's own
    name for the account (claude2...)."""
    note = (a.get("note") or "").strip()
    if note:
        return note
    email = (a.get("email") or "").strip()
    return email.split("@")[0] if email else a["account"]


def avatar_text(a):
    """The emoji the user picked (or the one matching a Mac symbol), else
    the initial."""
    icon = (a.get("icon") or "").strip()
    if icon in SYMBOL_EMOJI:
        return SYMBOL_EMOJI[icon]
    if icon and not re.match(r"^[a-z0-9.]+$", icon):
        return icon
    return display_name(a)[:1].upper()


def title_text(snap, failed, now):
    """Format.title: the current account's 5-hour use, LIMIT, or "!" on
    errors."""
    if failed:
        return "!"
    a = next((x for x in (snap or {}).get("accounts", []) if x["account"] == snap.get("current")), None)
    if not a:
        return ""
    if is_limited(a, now):
        return "LIMIT"
    if not a.get("fiveHour"):
        return ""
    return "%.0f%%" % win_percent(a["fiveHour"], now)


def tilde(path):
    return "~" + path[len(HOME):] if path and path.startswith(HOME) else (path or "—")


def valid_name(name):
    """The names `hotseat add` accepts."""
    return re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", name) is not None


# ---------- the hotseat CLI's answers ----------

def error_text(status, stderr):
    return stderr if stderr else "hotseat exited with %d" % status


def removal_problem(account, stdout):
    """What to tell the user after `hotseat remove <account> --yes --json`
    exited 0: None when all went well, else the Mac app's message."""
    try:
        result = json.loads(stdout)
        removed, logged_out, backup = result["removed"], result["loggedOut"], result["backup"]
    except (ValueError, KeyError, TypeError):
        removed = None
    if removed != account or not isinstance(logged_out, bool):
        return "hotseat printed something unexpected: invalid account removal result"
    if not logged_out:
        return "Hotseat removed %s, but Claude Code could not sign out. The account's files are in %s." % (
            account, backup)
    return None


def setup_steps(status, cli_version):
    """SetupStatus.steps() plus the CLI step, as tuples (id, title, done,
    detail, commands). status is `hotseat setup status --json`, or None when
    the CLI could not answer; commands is None for a step the app cannot
    run."""
    if cli_version:
        cli = ("cli", "Command line tool", True, "hotseat %s is installed" % cli_version, None)
    else:
        cli = ("cli", "Command line tool", False,
               "hotseat was not found in ~/.local/bin. Build it there: go build -o ~/.local/bin/hotseat ./cmd/hotseat",
               None)
    if status is None:
        return [cli]
    accounts, logins = status.get("accounts", 0), status.get("loginsFound", 0)
    conflicts, actions = status.get("shareConflicts", 0), status.get("shareActions", 0)
    desktop = status.get("desktop", "absent")
    share = "Links sessions, settings and skills of every account to ~/.claude. Close running claude sessions first."
    if conflicts > 0:
        share = ("%d files differ between accounts; ~/.claude's copy is used and the others stay where they "
                 "are (see: hotseat share plan)." % conflicts)
    return [
        cli,
        ("accounts", "Find your Claude logins", accounts > 0,
         "No Claude Code login found yet: run claude once in Terminal and log in, then click Check Again."
         if logins == 0 else "Registers the ~/.claude* dirs you are logged in to.", [["init"]]),
        ("share", "Share your work data", accounts > 0 and actions == 0, share, [["share", "apply", "--json"]]),
        ("shim", "Follow the current account", bool(status.get("shim")),
         "New terminals start claude as the account you pick", [["shim", "install"]]),
        ("tracking", "Usage and limit handoff", bool(status.get("statusline")) and bool(status.get("hook")),
         "Records usage from Claude Code's status line; offers another account when one runs out",
         [["statusline", "install"], ["hook", "install"]]),
        ("desktop", "Claude Desktop", desktop != "available",
         "Claude Desktop is not installed or never opened" if desktop == "absent"
         else "Lets Hotseat switch Desktop's account", [["desktop", "adopt"]]),
    ]


# ---------- launch at login ----------

def _exec_arg(arg):
    """One argument of a desktop entry's Exec key (Desktop Entry spec): quoted
    when it has a reserved character, with " ` $ and \\ escaped, and % doubled."""
    arg = arg.replace("%", "%%")
    if not re.search(r"""[\s"'\\><~|&;$*?#()`]""", arg):
        return arg
    return '"' + re.sub(r'(["`$\\])', r"\\\1", arg) + '"'


def autostart_entry(script):
    """The ~/.config/autostart entry that starts the app at login."""
    return ("[Desktop Entry]\nType=Application\nName=Hotseat\n"
            "Comment=Claude account switcher (top-bar icon)\n"
            "Exec=/usr/bin/python3 -I %s\nX-GNOME-Autostart-enabled=true\nNoDisplay=true\n" % _exec_arg(script))
