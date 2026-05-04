# peek

If you use Claude Code to debug your dev server, you've probably copy-pasted hundreds of stack traces. peek fixes that. It wraps your dev server and exposes its logs to Claude over MCP, so Claude reads what's happening directly.

```text
$ peek -- npm run dev
   ▲ Next.js 16
   - Local:   http://localhost:3000
   ✓ Ready in 312ms

# In another Claude Code session:
You:    my dev server is broken
Claude: [calls list_sessions, get_logs]
        TypeError at src/app/page.tsx:42. You're calling .map on
        undefined. Fired on the most recent compile.
```

## Install

### 1. Binary

```bash
curl -fsSL https://raw.githubusercontent.com/Pankaj3112/peek/main/install.sh | sh
```

Windows: download `peek_<version>_windows_<arch>.zip` from [Releases](https://github.com/Pankaj3112/peek/releases) and put `peek.exe` on your `%PATH%`.

### 2. Claude Code plugin

In a Claude Code session:

```
/plugin marketplace add Pankaj3112/peek
/plugin install peek
/reload-plugins
```

The plugin registers the MCP server and bundles a skill that teaches Claude when to reach for peek.

## How it works

`peek -- <command>` spawns your command under a pseudo-terminal, captures its output to `~/.peek/sessions/<ULID>/output.log`, and tees the bytes to your terminal. Your dev server feels identical to running it directly.

Three MCP tools (`list_sessions`, `get_logs`, `search_logs`) let Claude discover sessions by working directory and read or search the captured output. Claude finds your running server without being told the directory or session ID.

## Usage

```bash
peek -- npm run dev           # Node / Next / Vite / etc.
peek -- cargo run             # Rust
peek -- flask --app app run   # Python
```

Then in Claude Code, ask anything:

> "What's wrong with my dev server?"
>
> "Find any errors in my logs."
>
> "Did the last request return an error?"

Claude calls `list_sessions` (filtered by your cwd), then `get_logs` or `search_logs` to read the captured output.

## Project integration

For a project where you want every dev session captured, wire peek into your dev script:

```jsonc
// package.json
"scripts": {
  "dev": "peek -- next dev"
}
```

Equivalent for `Justfile`, `Makefile`, `scripts/dev.sh`. **Wrap dev-mode commands only. Never `start`, `build`, `test`, or anything CI runs.** peek is a development tool; production startup and CI builds shouldn't depend on it.

## CLI reference

```bash
peek list             # human-readable table of sessions
peek logs <id>        # tail or dump a session (id prefix works)
peek --version
peek --help
```

## Limitations

- **peek doesn't manage your server.** It wraps and observes; it doesn't restart, signal, or attach to existing processes. To restart, Ctrl+C and run again. peek picks up the new session.
- **All data stays local.** No network calls beyond the MCP connection to your Claude client. No telemetry, no syncing, no hosted dashboard.
- **Logs are capped at 100 MB per session.** Older lines roll out. For very verbose servers, only recent context is preserved.
- **MCP is request/response, not streaming.** For "watch this server", Claude polls `get_logs` periodically. The bundled skill documents the pattern.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `command not found: peek` | `~/.local/bin` is not on your `PATH`. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc and reload. |
| `peek mcp failed to start` (in Claude Code) | Same cause: `peek` is not on the `PATH` Claude Code inherits. Fix `PATH`, then restart Claude Code. |
| `'peek' is not recognized…` (Windows) | Download the `.zip` from [Releases](https://github.com/Pankaj3112/peek/releases) and put `peek.exe` on your `%PATH%`. |
| Claude can't find your running session | Ask Claude to call `list_sessions` with no `cwd` filter. Symlinks (e.g. macOS `/tmp` → `/private/tmp`) aren't resolved; peek matches literal paths. |

## Uninstall

In Claude Code:

```
/plugin uninstall peek
/plugin marketplace remove peek
```

Then in a terminal:

```bash
rm ~/.local/bin/peek
rm -rf ~/.peek    # optional, your captured session data
```

## Power-user notes

**Manual MCP wiring without the plugin** (no bundled skill, just the three MCP tools):

```bash
claude mcp add peek -- peek mcp
```

**Build from source:**

```bash
git clone https://github.com/Pankaj3112/peek.git
cd peek
make build
cp bin/peek ~/.local/bin/peek
```

**Codex CLI:** the plugin manifest at `plugins/peek/.codex-plugin/plugin.json` is valid for Codex's marketplace. Refer to your Codex docs for the exact install command.

## Development

```bash
make build   # → bin/peek
make test
make vet
make fmt
```

Design spec and implementation plan live in `docs/superpowers/`. The repo layout is documented in the spec.

---

peek is MIT licensed. Issues and PRs welcome.
