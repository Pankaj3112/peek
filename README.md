# peek

Wrap your dev server. Read its logs from Claude.

## What is peek?

peek is a CLI you wrap your dev server with. It captures stdout and stderr to disk and exposes them to Claude via MCP, so Claude can see what's happening without you pasting output. Run `peek -- npm run dev` instead of `npm run dev`, and Claude in another terminal can debug your server without being told the working directory or session ID.

## Install

> **Note:** No release is tagged yet. Until v0.1.0 ships, build from source (see [Development](#development) below). The curl install will work once a release is published.

**After v0.1.0 ships — one command:**

```bash
curl -fsSL https://raw.githubusercontent.com/Pankaj3112/peek/main/install.sh | sh
```

This installs the `peek` binary to `~/.local/bin` (Linux/macOS) or via Scoop on Windows.

**Until then — build from source:**

```bash
git clone https://github.com/Pankaj3112/peek.git
cd peek
make build
# Binary is at bin/peek — copy it somewhere on your PATH:
cp bin/peek ~/.local/bin/peek
```

## Quick Start

```bash
# 1. Install the binary (see above)

# 2. Wire peek into Claude Code
claude mcp add peek -- peek mcp

# 3. Wrap your dev server
peek -- npm run dev
```

Claude now has access to your server's logs. Ask it: "What's wrong with my dev server?"

## Plugin Install (Marketplace)

If you prefer the plugin marketplace path over manual MCP wiring:

**Claude Code:**

```
/plugin marketplace add Pankaj3112/peek
/plugin install peek
```

**Codex CLI:**

```
codex plugin install Pankaj3112/peek
```

Confirm the MCP server is reachable after install with `/mcp` (Claude Code) or the Codex equivalent.

## Manual MCP Wiring

For users who don't want the plugin marketplace:

```bash
claude mcp add peek -- peek mcp
```

This adds peek as a local MCP server scoped to Claude Code. The server starts on demand when Claude needs it.

## Usage

**Wrap a dev server:**

```bash
peek -- npm run dev
```

Output is forwarded to your terminal exactly as normal. peek writes a session record under `~/.local/share/peek/` (Linux/macOS) or `%LOCALAPPDATA%\peek\` (Windows).

**List active and recent sessions:**

```bash
peek list
```

```
ID                         STATUS   CWD
01HXYZ...                  running  /home/user/myapp
01HABC...                  exited   /home/user/oldapp
```

**Read logs from a session:**

```bash
peek logs 01HXYZ...
```

You can also pass a partial ID prefix — peek resolves unambiguous prefixes.

**From Claude (via MCP tools):**

Claude calls `list_sessions`, `get_logs`, and `search_logs` directly. You don't need to run `peek list` or `peek logs` yourself unless you want to inspect sessions manually.

## What peek does NOT do

These are intentional constraints, not missing features:

- **Restart your server.** Run the command yourself. peek wraps it; it does not manage it.
- **Detect your framework.** Claude reads the logs and figures it out.
- **Name your sessions.** The ULID and cwd identify them. Named sessions are a v2 consideration.
- **Forward logs to a hosted dashboard.** All data stays local. No network calls beyond Claude's MCP connection.
- **Filter logs server-side by log level or timestamp.** `search_logs` supports regex. Everything else is Claude's job.
- **Stop, restart, or signal your process.** No `peek stop`, no `peek restart`.
- **Detect ports, git roots, or package names.** Out of scope for v1.
- **Provide a web UI, TUI, or GUI.** The interface is Claude.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `command not found: peek` | `~/.local/bin` is not on your PATH. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc file and reload. |
| `peek mcp failed to start` (Claude Code error) | Same cause: `peek` is not on the PATH that Claude Code uses. Fix PATH as above, then restart Claude Code. |
| `'peek' is not recognized as an internal or external command` (Windows) | Install via Scoop: `scoop install peek` (available after v0.1.0 ships). Until then, build from source and add the binary to a directory on your `%PATH%`. |
| Claude says it can't find any sessions | Make sure `peek -- <command>` is running in a terminal before asking Claude. Sessions appear immediately on start. |
| Logs are truncated | peek enforces a 50 MB soft / 100 MB hard cap per session. If your server is very verbose, older lines are rotated out. |

## Development

```bash
git clone https://github.com/Pankaj3112/peek.git
cd peek
make build   # produces bin/peek
make test    # runs all unit and integration tests
make vet     # go vet
make fmt     # gofmt -s -w
```

**Repo layout:**

```
cmd/peek/            # main binary entrypoint
cmd/peek-capture/    # dev tool: capture raw ANSI output for test fixtures
internal/ansi/       # ANSI stripping and line discipline
internal/cli/        # peek list, peek logs subcommands
internal/mcp/        # MCP server (list_sessions, get_logs, search_logs)
internal/platform/   # OS-specific path helpers and ConPTY wiring
internal/store/      # session index and log storage
internal/wrapper/    # pty/pipe wrapping, process lifecycle
plugins/peek/        # Claude Code and Codex plugin manifests
```

**Capturing new ANSI fixtures:**

`cmd/peek-capture/` is a dev-only tool for recording real terminal output to use as golden test fixtures in `internal/ansi/`. Run it against a command that emits the ANSI sequences you want to cover, then commit the output file alongside a test in `internal/ansi/testdata/`.

```bash
go run ./cmd/peek-capture -- <command that emits ANSI>
```

**Golden file updates:**

```bash
make golden-unix   # regenerate unix golden files (macOS/Linux only)
```

---

peek is MIT licensed. Issues and PRs welcome.
