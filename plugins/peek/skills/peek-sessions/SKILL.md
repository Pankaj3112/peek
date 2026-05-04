---
name: peek-sessions
description: Use when the user asks about a running dev server, build, local error, or any foreground process — peek captures session output to disk and exposes it via MCP, so you can read without the user copy-pasting. Also covers wiring peek into project run scripts (package.json, Procfile, Justfile, Makefile) so capture is always-on.
---

# Reading peek sessions

peek wraps a dev server (or any foreground process) and captures its output to disk. The peek MCP tools let you read those logs without the user copying anything. If peek is installed, treat it as the first place to look when debugging local processes.

## When this skill applies

Use peek tools whenever the user:

- Asks "what's happening with my server" / "why isn't this working" / "is the dev server up"
- Reports an error from local development without pasting the full output
- Asks to debug a 500, build failure, crash, or hang
- Mentions "the dev server", "running locally", "the build", or any long-running command
- Talks about a process they started in another terminal

If the user hasn't run anything under `peek -- ...`, there will be no sessions for their cwd. In that case, suggest they wrap their next run, or use one of the project-wiring patterns below to make capture always-on.

## Available tools

**`list_sessions(cwd?: string)`** — lists captured sessions. Pass `cwd` to filter to sessions related to a specific directory; the matcher returns sessions where the session's cwd is exact, an ancestor of, or a descendant of the query path. Without `cwd`, returns all sessions newest-first.

**`get_logs(id: string, lines?: number, start_line?: number)`** — read log lines from a session. Default tails the last 100 lines (max 1000). Use `start_line` to read forward from a specific line number — useful for the polling pattern below.

**`search_logs(id: string, pattern: string, context?: number, max_matches?: number)`** — Go RE2 regex search per line. Returns grep -C style output with `:` for matched lines, `-` for context lines, `--` between non-adjacent groups. Default 3 lines context (max 50), 50 max matches (max 200). Prefix the pattern with `(?i)` for case-insensitive, `(?m)` for multi-line.

## Common patterns

**"What's happening?"** Call `list_sessions` with the user's cwd if you have it, then `get_logs(id)` to read the tail. Identify running sessions by `status: "running"` and absence of `wrapper_died`.

**"Find the error"** Call `search_logs(id, "(?i)(error|exception|failed|traceback|fatal|panic)")`. Vary the regex by language: Python adds `Traceback`, Rust adds `panicked`, JS adds `TypeError|ReferenceError`, Go adds `goroutine .* runtime error`. Use `context: 5` for stack traces.

**"What just broke?"** When there's both old noise and recent errors, search with `max_matches: 5` to surface only the most recent matches. The default returns oldest-first; for "newest only", combine a strict cap with `last_match` in the response — the response's `last_match` field summarizes the newest match when results are truncated.

**"Watch the server"** peek's MCP layer is request/response, not streaming. To poll for new lines:

1. `get_logs(id, lines: 100)` — note `total_lines` in the response
2. Wait several seconds (or whenever the user expects something to have happened)
3. `get_logs(id, start_line: total_lines + 1)` — returns only lines added since
4. Update your local `total_lines` to the new response's value, repeat as needed

There is no persistent connection. Each call is a fresh request. If the user wants live tail behavior, this is the substitute.

**"Is the server still up?"** Check `session_status` in the `get_logs` response. `running` means alive. `exited` with `wrapper_died: true` means peek itself died (SIGKILL, OOM, machine reboot) — the wrapped server's actual exit is unknown but the log up to that point is intact. `exited` with a non-null `exit_code` means the wrapped process exited normally with that code (or 128+signum if signaled). `exited` with null `exit_code` and no `wrapper_died` shouldn't normally happen.

**Picking among multiple sessions** When `list_sessions` returns several, prefer the running session for "what's happening now." For "what failed earlier today", search across all candidate sessions — the on-disk log is intact even after exit. The per-cwd ring buffer caps at 3 sessions; older ones are evicted, so don't reference very old sessions, they may be gone.

## Wiring peek into project workflows

**peek is a development-time dependency only.** Only wrap commands developers run on their own machines while iterating. Never wrap production startup, deploy, or CI build commands — that would force every contributor and every CI runner to install peek, and would capture logs that already go to better destinations (PaaS dashboards, CI artifacts).

Below are patterns by file type. Pick based on whether peek should be required for the dev script or available alongside a peek-free fallback for contributors who don't have it installed.

### `package.json` (Node, Bun, pnpm, yarn)

Wrap **only** dev-mode scripts. Do NOT wrap `start`, `build`, `test`, `lint`, or anything CI runs.

**Required — committed to project:**
```json
"scripts": {
  "dev": "peek -- next dev"
}
```

Document peek as a recommended dev tool in the README so contributors install it before running `npm run dev`.

**Optional — graceful fallback so contributors without peek aren't blocked:**
```json
"scripts": {
  "dev": "command -v peek >/dev/null && peek -- next dev || next dev",
  "dev:peek": "peek -- next dev",
  "dev:raw": "next dev"
}
```

Or keep `dev` as the canonical no-peek command and add `dev:peek` alongside; contributors choose explicitly.

### Rust (`Justfile` / `Makefile`)

Cargo has no script section, but project task runners do. Wrap `cargo run` only — leave `cargo build`, `cargo test`, `cargo check` alone.

```makefile
.PHONY: dev dev-raw
dev:
	peek -- cargo run

dev-raw:
	cargo run
```

Justfile equivalent:
```just
dev:
    peek -- cargo run

dev-raw:
    cargo run
```

### Python (`Makefile`, `Justfile`, or `scripts/dev.sh`)

Most Python projects don't standardize a `dev` command. A `Makefile` target or a one-line shell script is the usual home:

```makefile
dev:
	peek -- flask --app app run --debug
```

```sh
# scripts/dev.sh
#!/bin/sh
exec peek -- flask --app app run --debug
```

### What NOT to wrap

- **`start` / `serve` / production startup scripts.** peek is a dev tool. Production servers should write to wherever the platform expects (stdout for containers, syslog, journald, the PaaS log stream).
- **`build` / `compile` / bundling scripts.** Build steps run in CI and locally as one-shot tasks. CI already captures their output; local one-shot builds don't need session capture.
- **`test`, `lint`, `typecheck`, `format`, or any CI-run command.** These are batch tasks that flood the 100 MB session cap quickly and aren't useful to read back via MCP.
- **`docker-compose` / container orchestrators.** peek wraps the host-side stdout, not what runs inside containers. Use `docker logs` for container output.
- **Heroku/Render/Fly Procfiles or any deploy manifests.** These run on remote infrastructure where peek isn't installed and shouldn't be a dependency.

## Limitations to keep in mind

peek does not:

- **Restart, stop, or signal the wrapped process.** Read-only from the MCP side. If the user wants to restart a server, ask them to do it manually.
- **Stream live output.** Use the polling pattern above.
- **Detect frameworks or parse log formats.** You read raw text and reason about it.
- **Filter logs server-side beyond regex.** Use `search_logs` for any filtering; don't suggest server-side timestamp filtering or level filtering — that's not exposed.
- **Persist sessions beyond the per-cwd ring buffer.** Maximum 3 sessions per cwd; the 4th evicts the oldest.
- **Resolve symlinks in cwd matching.** If the user runs peek inside a symlinked directory, the cwd stored is the literal path; ancestor/descendant matching is byte-level.

## Things to actively avoid suggesting

- "Tail this log file directly" — defeats the purpose of MCP integration; just call `get_logs`.
- "Restart the server and try again" if you haven't yet read the logs to understand what failed.
- A skill or tool to "watch and notify" — use the polling pattern instead. peek doesn't push.
- Attaching peek to a process that's already running. peek must be the wrapper from start; you cannot attach to an existing PID.

## Quick reference

| Goal | Tool call |
|------|-----------|
| Find sessions for current dir | `list_sessions(cwd: <path>)` |
| Read tail of a session | `get_logs(id)` (last 100 lines) |
| Read a wider tail | `get_logs(id, lines: 500)` |
| Read a specific range | `get_logs(id, lines: 50, start_line: 1200)` |
| Find errors with stack trace | `search_logs(id, "(?i)(error|exception|traceback)", context: 10)` |
| Most recent N matches | `search_logs(id, pattern, max_matches: 5)` then check `last_match` |
| Poll for new content | `get_logs(id, start_line: prev_total + 1)` repeatedly |
