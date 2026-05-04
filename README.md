# peek

Wrap your dev server. Read its logs from Claude.

## What is peek?

peek is a CLI you wrap your dev server with. It captures stdout and stderr to disk and exposes them to Claude (or any MCP client) via three tools, so Claude can see what's happening without you pasting output. Run `peek -- npm run dev` instead of `npm run dev`, and Claude in another terminal can debug your server without being told the working directory or session ID.

## Install

Two steps: install the binary, then wire it into your Claude client.

### 1. Install the binary

**macOS / Linux** — one command:

```bash
curl -fsSL https://raw.githubusercontent.com/Pankaj3112/peek/main/install.sh | sh
```

This drops `peek` at `~/.local/bin/peek`. Make sure `~/.local/bin` is on your `PATH` — the installer prints shell-specific instructions if it isn't.

**Windows** — download `peek_<version>_windows_<arch>.zip` from [GitHub Releases](https://github.com/Pankaj3112/peek/releases) and extract `peek.exe` to a directory on your `%PATH%`. (Homebrew tap and Scoop bucket distribution are planned for a later release.)

**From source** (any platform with Go installed):

```bash
git clone https://github.com/Pankaj3112/peek.git
cd peek
make build
cp bin/peek ~/.local/bin/peek
```

Verify the install:

```bash
peek --version   # → peek 0.1.0
```

### 2. Wire peek into Claude Code

Two options. Pick one.

#### Option A: Plugin marketplace (recommended)

Includes the bundled `peek-sessions` skill, which helps Claude know when to reach for peek (when the user asks about a dev server, build, error, or running process) and documents project-wiring patterns for `package.json`, `Justfile`, etc.

In a Claude Code session:

```
/plugin marketplace add Pankaj3112/peek
/plugin install peek
/reload-plugins
```

That's it. The plugin auto-registers the MCP server and the skill.

#### Option B: Manual MCP wiring

No plugin, no skill — just the three MCP tools.

```bash
claude mcp add peek -- peek mcp
```

Restart your Claude Code session.

### 3. Verify

In Claude Code:

```
/mcp
```

You should see `peek` listed with three tools: `list_sessions`, `get_logs`, `search_logs`.

### Codex CLI

Codex's plugin marketplace works similarly. The plugin manifest we ship is valid for Codex (see `plugins/peek/.codex-plugin/plugin.json`); refer to your Codex CLI docs for the exact install command.

## Use it

Wrap any dev server:

```bash
peek -- npm run dev
```

Output is forwarded to your terminal exactly as normal. peek captures it to `~/.peek/sessions/<ULID>/output.log` while it runs.

In another Claude Code session at the same project root, ask anything about your server:

> "What's wrong with my dev server?"
>
> "Find any errors in my logs."
>
> "Did the last request return an error?"

Claude calls `list_sessions` to find your running session (filtered by cwd), then reads with `get_logs` or `search_logs`. You don't tell it the directory or session ID.

### Always-on capture (project-wiring)

For a project where you always want peek capturing during local dev, wire it into your run scripts. **Wrap dev-mode commands only — never `start`, `build`, `test`, or anything CI runs.**

```jsonc
// package.json
"scripts": {
  "dev": "peek -- next dev"
}
```

For graceful fallback when a contributor doesn't have peek installed:

```jsonc
"scripts": {
  "dev": "command -v peek >/dev/null && peek -- next dev || next dev",
  "dev:peek": "peek -- next dev",
  "dev:raw":  "next dev"
}
```

The bundled skill documents wiring for Cargo, Python, Justfile, Makefile, etc.

## CLI commands

These run on your terminal — Claude doesn't need them, but they're useful for inspecting sessions yourself.

```bash
peek list                 # human-readable table of sessions
peek logs <id>            # tail or dump a session's logs
peek logs <id-prefix>     # unambiguous prefixes work too
```

Example `peek list` output:

```
ID         STATUS      STARTED               CMD              CWD
01H8XHS7Q  running     2026-05-04 14:23:01   npm run dev      ~/Code/myapp
01H7YGC2R  exited(0)   2026-05-04 12:10:33   cargo run        ~/Code/otherproj
01H6ABC12  exited(?)   2026-05-04 09:00:00   next dev         ~/Code/myapp
```

`exited(?)` means the wrapper itself died (SIGKILL, machine reboot) — the wrapped process's exit is unknown but the captured log up to that point is intact.

## What peek does NOT do

These are intentional v1 constraints, not missing features:

- **Restart your server.** Run the command yourself. peek wraps; it does not manage.
- **Detect your framework.** Claude reads the logs and figures it out.
- **Name your sessions.** The ULID and cwd identify them.
- **Forward logs anywhere.** All data stays local. No network calls beyond the local MCP connection.
- **Filter logs server-side by log level or timestamp.** `search_logs` does regex; everything else is Claude's job.
- **Stop, restart, or signal your process.** No `peek stop`, no `peek restart`.
- **Detect ports, git roots, or package names.** Out of scope.
- **Stream live output.** MCP is request/response; for "watch this", Claude polls `get_logs(start_line: ...)` periodically. The bundled skill documents the pattern.
- **Provide a web UI, TUI, or GUI.** The interface is Claude.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `command not found: peek` | `~/.local/bin` is not on your `PATH`. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc and reload. |
| `peek mcp failed to start` (Claude Code error) | Same cause: `peek` is not on the `PATH` Claude Code uses. Fix `PATH`, then restart Claude Code. |
| `'peek' is not recognized as an internal or external command` (Windows) | Download the Windows `.zip` from [GitHub Releases](https://github.com/Pankaj3112/peek/releases) and extract `peek.exe` to a directory on your `%PATH%`. |
| Claude says it can't find any sessions | Make sure `peek -- <command>` is running in a terminal before asking. Sessions appear immediately on start. Also try asking with no `cwd` filter — your project root might differ from Claude's working directory. |
| Claude is calling `list_sessions` but not finding my running server | Confirm the cwd matches: run `pwd` where you started peek, and compare against the cwd Claude is querying. Symlinks (e.g. macOS `/tmp` → `/private/tmp`) are not resolved — peek matches literal paths. |
| Logs are truncated | peek caps each session at 50 MB current + 50 MB rotated = 100 MB max. For very verbose servers, older lines roll out. |
| The plugin install picks up the wrong peek binary | The MCP server uses whichever `peek` is on `PATH` when Claude Code spawns it. Run `which peek` outside Claude Code to confirm; if you have multiple installations, remove the unwanted one (see Uninstall below). |

## Uninstall

### Remove from Claude Code

If you installed via the plugin:

```
/plugin uninstall peek
/plugin marketplace remove peek
```

If you wired peek manually with `claude mcp add`:

```bash
claude mcp remove peek
```

After either, restart your Claude Code session (or run `/reload-plugins` if you only want to drop the plugin).

### Remove the binary

```bash
rm ~/.local/bin/peek
```

(Or wherever you installed it — `which peek` will show the path.)

### Remove captured session data (optional)

```bash
rm -rf ~/.peek
```

That's everything peek touches on disk. No system services, no daemons, no config files anywhere else.

## Development

```bash
git clone https://github.com/Pankaj3112/peek.git
cd peek
make build   # → bin/peek
make test    # all unit and integration tests
make vet     # go vet
make fmt     # gofmt -s -w
```

### Repo layout

```
cmd/peek/                                # main binary
cmd/peek-capture/                        # dev tool: capture raw ANSI for golden fixtures
internal/ansi/                           # ANSI stripping + line discipline (\r-as-redraw)
internal/cli/                            # peek list, peek logs
internal/mcp/                            # MCP server: list_sessions, get_logs, search_logs
internal/platform/                       # path helpers + cross-platform PID liveness
internal/store/                          # session metadata + log writer + ring buffer
internal/wrapper/                        # pty wrapping, signal forwarding, lifecycle
plugins/peek/                            # Claude Code + Codex plugin manifests + skill
docs/superpowers/specs/                  # v1 design spec
docs/superpowers/plans/                  # implementation plan
```

### Capturing new ANSI fixtures

`cmd/peek-capture/` records raw pty output to a file for use as test fixtures in `internal/ansi/testdata/`. Run it against any tool whose ANSI behavior you want covered:

```bash
go run ./cmd/peek-capture -o internal/ansi/testdata/unix/some-tool.bin -- some-tool args...
```

### Regenerating golden test outputs

```bash
make golden-unix   # macOS/Linux only
```

After regenerating, **inspect each `*-expected/*.txt` by hand** to confirm the output is what you expect — golden files are hand-curated, not blindly accepted.

---

peek is MIT licensed. Issues and PRs welcome.
