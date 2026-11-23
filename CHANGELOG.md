# Changelog

## Unreleased

- Limit messages that go on with a sentence after the time zone, such as model-scoped limits ("You've hit your Fable limit · resets Nov 29 at 11am (Asia/Shanghai). Switch to another model …"), now get their reset time instead of an open-ended limit.

## 0.1.0

First public release.

- CLI: registers accounts, shares work data, adds a `claude` shim, records usage from the status line, hands off at limits, switches Claude Desktop, and adds, removes, notes and icons accounts. Includes `setup status`, `uninstall` and `doctor`.
- Menu bar app: account cards with usage, one-click switching (both sides, CLI only or Desktop only), Settings, and a first-run Setup window. It carries its own CLI.
- Release builds: universal CLI and app, and a Homebrew formula that builds from source.
