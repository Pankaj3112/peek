# Peek v1 Design Spec

**Date:** 2026-05-03
**Status:** Approved for implementation

## Overview

A CLI you wrap your dev server with. It captures logs to disk and exposes them to Claude via MCP, so Claude can read what's happening without spawning its own servers.

The product in one sentence: `peek -- <command>` runs `<command>`, tees its output to your terminal and to a log file, and a separate `peek mcp` MCP server lets Claude read those logs.

## Design Constraints

- Small core. No process control (no restart, no stop). If you want that, run the command yourself.
- No framework or language detection. Claude reads logs and figures out what's happening, the same way a human does.
- All data is local. No daemon, no IPC socket, no hosted dashboard.
- The constraint is the feature. v2 conversations are captured at the bottom of this doc.

## CLI Surface

```
peek -- <cmd> [args...]    Wrap and run a command
peek mcp                   Start MCP server over stdio
peek list                  Human-readable session list
peek logs <id>             tail-f a session (or dump if exited)
peek --version
peek --help
```

Five subcommands. The only flags peek itself accepts are `--version` and `--help`. The `--` is **mandatory** in the wrap form — it separates peek's args from the wrapped command's args and removes ambiguity around any flags the wrapped command takes (`peek -- next dev --turbo` is unambiguous; `peek next dev --turbo` would force flag-parsing heuristics that aren't worth the complexity).

## Wrapper Runtime

`peek -- <cmd>` is the heart of the product. It must feel identical to running `<cmd>` directly — anything less and users uninstall before the MCP magic earns its keep.

### PTY

The child runs under a pseudo-terminal. Library: `github.com/aymanbagabas/go-pty` (Unix via creack/pty, Windows via ConPTY, single API).

Initial pty size is set on creation from the parent terminal's current size — not just on the first SIGWINCH. Otherwise the child starts at 80×24 regardless of the user's terminal.

### Stdin forwarding

The parent terminal is put into raw mode using `golang.org/x/term`. The raw-mode enable call is wrapped with a `defer term.Restore(...)` immediately after, so panics or unexpected exits don't leave the user's terminal broken.

A goroutine does `io.Copy(ptyMaster, os.Stdin)`. This is what makes `next dev`'s `r` (restart) and `q` (quit) keys work.

### Signal forwarding

Explicit forwarding (not "let the kernel do it via process group"):

| Signal | Behavior |
|--------|----------|
| SIGINT | Forward to child. Start 5s graceful timer. If a second SIGINT arrives within the window, send SIGKILL to child immediately (skip the wait). |
| SIGTERM | Forward to child. Start 5s graceful timer. SIGKILL if not exited. |
| SIGHUP | Same pattern as SIGTERM (parent disconnect / SSH session close). Do **not** behave like nohup — closing the terminal kills the wrapped server, exactly as it would running directly. |
| SIGQUIT | Forward to child. |
| SIGTSTP (Ctrl+Z) | Forward SIGTSTP to child first, then suspend the wrapper. Without this, Ctrl+Z leaves the dev server running invisibly. |
| SIGCONT (fg) | Forward SIGCONT to child first, then resume the wrapper. |
| SIGWINCH | Update pty master size via `ioctl(TIOCSWINSZ)` with the parent terminal's new size. |

Graceful shutdown timeout is 5 seconds, no flag.

Out of scope for v1: signal masking, SIGPIPE handling, nohup-style detachment.

### Exit code propagation

Wrapper exits with the child's exit code on normal exit, or `128 + signum` on signal death (matches bash convention). The same value is persisted to `meta.json` as `exit_code`.

### Environment

The child sees `PEEK_SESSION_ID=<ulid>` in its environment. Lets the child or anything it spawns reference its own session. Free.

### Shutdown order

`meta.json` (with `exit_code` and `exited_at`) is written **before** the wrapper itself exits. Order matters — otherwise concurrent readers can see "exited" status with null exit_code and misclassify a clean exit as a wrapper-died case.

## Session Store

```
~/.peek/sessions/<ulid>/
├── meta.json
├── output.log
└── output.log.1     (only if rotated)
```

The session directory is created by the wrapper on start, before the child is spawned. ULID is generated at that point using `github.com/oklog/ulid`. ULIDs are 26 characters, time-ordered, lexicographically sortable; sorting directory entries by name yields chronological order for free.

### `meta.json` schema

```json
{
  "version": 1,
  "id": "01H8XHS7Q9M2K3F4P5N6R7T8V9",
  "pid": 47523,
  "cwd": "/Users/pankaj/Code/myapp",
  "cmd": ["npm", "run", "dev"],
  "started_at": "2026-05-03T14:23:01.234Z",
  "status": "running",
  "exited_at": null,
  "exit_code": null
}
```

| Field | Type | Notes |
|-------|------|-------|
| `version` | int | Always `1` in v1. Future-proofs schema migration. |
| `id` | string | ULID, 26 chars. Matches the directory name. |
| `pid` | int | Wrapper's PID (not child's). Used for crash detection at read time. |
| `cwd` | string | Absolute path. `filepath.Clean`'d. Set from `os.Getwd()` at start. |
| `cmd` | string[] | argv array. No shell parsing. |
| `started_at` | string | RFC3339Nano UTC, e.g. `2026-05-03T14:23:01.234Z`. Format matches the on-disk log line prefix exactly so Claude can string-correlate. |
| `status` | enum | `running` (file says wrapper is alive) or `exited` (wrapper finished). Only two states stored on disk. |
| `exited_at` | string \| null | RFC3339Nano UTC. Null while running. |
| `exit_code` | int \| null | Child's exit code (0-255 normal, 128+N for signals). Null while running. Null on `exited` status indicates "wrapper died, we don't know how the child ended" — see crash detection. |

### Crash detection (read-time, virtual)

When any reader (`peek list`, `list_sessions`, `get_logs`, `search_logs`) loads a `meta.json` with `status: "running"`, it issues a `kill(pid, 0)` (Unix) or `OpenProcess` (Windows). If the wrapper PID is not alive:

- The session is reported with `status: "exited"`, `exit_code: null`, `exited_at: null`.
- The on-disk file is **not mutated**. Reads stay idempotent.

PID-reuse is a theoretical risk (another process inherited the dead wrapper's PID). Mitigation deferred — real-world risk over a session lifetime is tiny. If it surfaces, add `started_at` vs process-creation-time comparison in v2.

## Log Format on Disk

Single merged log file per session: `output.log`. Rotated to `output.log.1` when it crosses 50 MB. Hard ceiling per session: 100 MB.

### Why one file

A pty merges stdout and stderr at the kernel level. Using a pty for both (which we want, for tty-detecting tools) means the parent receives a single byte stream — no reliable way to recover the split without losing tty behavior. We commit to merged. Claude reads severity from message content the same way humans do.

### What gets written

The pty master fd produces a raw byte stream. Two transformations happen on the path to disk only — the user's terminal gets the raw bytes untouched (colors, spinners, TUIs all unmodified):

1. **ANSI escape sequences are stripped.** Regex: `\x1b\[[0-9;]*[a-zA-Z]` (CSI sequences) plus equivalents for OSC and a few other escape forms. Library or hand-rolled — both fine.
2. **Carriage returns drive line redraw.** When `\r` arrives without a following `\n`, the bytes that follow overwrite the in-memory current line. Only newline-terminated lines (`\n` or `\r\n`) are flushed to disk. Spinners that emit thousands of `\r` repaints collapse to their final state. TUIs become much quieter.

### Line format on disk

Each finalized line is prefixed with an RFC3339Nano UTC timestamp (millisecond precision) and a single space:

```
2026-05-03T14:23:01.234Z compiling src/foo.ts
2026-05-03T14:23:01.512Z ERROR: failed to compile
```

Format matches `meta.json`'s `started_at` exactly so Claude can string-compare.

### Rotation

The size check runs inline after each flushed line — no background goroutine, so there's no race on the file descriptor. When `output.log` reaches 50 MB:

1. Close the current file descriptor.
2. `rename(output.log, output.log.1)` (overwriting any existing `.1`).
3. Open a fresh `output.log` for append and continue writing.

`get_logs` and `search_logs` span both files as one conceptual stream with global line numbers (line 1 = first line of `.1`, continuing into `output.log`). When `.1` does not exist, line numbering starts at 1 in `output.log`.

No configurability. Documented in README.

### Test data

ANSI handling tests are driven by **golden files**, not hand-written cases. Capture real output from `next dev`, `vite`, `npm install`, `cargo build`, `flask run`, and similar tools into `internal/ansi/testdata/`. Test against captured **Windows pty output too**, not just Unix — Windows ConPTY emits subtly different escape patterns.

## Ring Buffer

Per-cwd: keep at most 3 sessions. Oldest evicted when a 4th starts.

### Trigger

At wrapper start, before writing the new session's `meta.json`:

1. Scan `~/.peek/sessions/`.
2. For each entry, read its `meta.json` (skip silently on `ENOENT` or parse errors — sessions can vanish under us).
3. Filter to entries with matching `cwd`.
4. Sort by ULID ascending (oldest first).
5. If count ≥ 3, `os.RemoveAll` on the oldest entries until count = 2.
6. Create the new session directory.

### Concurrency

**Lock-free, idempotent.** No global lock file. Two simultaneous wrappers in the same cwd may both decide to evict the same session — one succeeds, the other gets `ENOENT` from `RemoveAll` and ignores it. Worst case: 4 sessions briefly until the next wrapper starts.

### Failure mode

If `RemoveAll` fails for any other reason (stale NFS mount, permission), log a warning to stderr and continue starting the new session. Eviction is best-effort; new session creation is not blocked on it.

### Reader robustness

`peek list`, all three MCP tools, and any other path that scans the session store must:

- Tolerate `ENOENT` mid-scan (skip silently).
- Tolerate parse errors on `meta.json` (skip silently with optional debug log).

## MCP Server

`peek mcp` runs over stdio. On startup, log version and binary path to stderr — Claude Code's `/mcp` debug view surfaces these and reveals install issues immediately.

Three tools.

### `list_sessions(cwd?: string)`

Returns sessions whose `cwd` is related to the query `cwd`: exact match, ancestor of query (Claude is deeper than the session), or descendant of query (session is in a subdirectory of where Claude is). When `cwd` is omitted, returns all sessions.

**Path matching:**
- `filepath.Clean` both sides.
- Separator-aware prefix check: `/home/foo` does **not** match `/home/foobar`.
- **Don't resolve symlinks.** Match canonical paths only.
- **Case-sensitive comparison everywhere**, including macOS APFS and Windows. Document the gotcha.

**Status filter:** none. Return both `running` and `exited`. Per-cwd ring buffer caps total to ~3 per cwd anyway.

**Sort:** ULID descending (newest first).

**Response:**

```json
{
  "sessions": [
    {
      "id": "01H8XHS7Q9M2K3F4P5N6R7T8V9",
      "cwd": "/Users/pankaj/Code/myapp",
      "cmd": ["npm", "run", "dev"],
      "started_at": "2026-05-03T14:23:01.234Z",
      "status": "running",
      "exited_at": null,
      "exit_code": null
    }
  ]
}
```

`status` reflects crash detection (virtual `exited` when wrapper PID is dead).

**`wrapper_died` field (conditional):** when crash detection fires (file said `running` but PID is dead), the session object includes `"wrapper_died": true`. Field is omitted otherwise. The redundancy with null `exit_code` on virtual `exited` is the point — it lets Claude (and humans reading the response) distinguish "the dev server crashed" (non-null exit_code) from "peek itself died" (`wrapper_died: true`) without encoding a rule about null exit_code semantics.

### `get_logs(id: string, lines?: number, start_line?: number)`

**Parameters:**
- `id`: ULID. Exact match required (no prefix matching for MCP — only `peek logs <id>` accepts prefixes).
- `lines`: default 100, max 1000.
- `start_line`: optional, 1-indexed. If omitted, tail the last `lines` lines. If set, return up to `lines` lines starting at that line (fewer if not enough remain). Negative values → error. `start_line > total_lines` → empty result with accurate `total_lines`, **no error** (LLMs occasionally pass stale numbers; erroring wastes a turn).

**Scope:** spans `output.log.1` and `output.log` as one stream with global line numbers.

**Response:**

```json
{
  "lines": "1231: 2026-05-03T14:23:01.230Z compiling src/foo.ts\n1232: 2026-05-03T14:23:01.231Z compiling src/bar.ts\n",
  "from_line": 1231,
  "to_line": 1330,
  "total_lines": 1330,
  "session_status": "running"
}
```

`lines` is a single newline-joined string, not an array. Format: `<line_num>: <iso_timestamp> <text>`. `session_status` lets Claude answer "is this still running?" without an extra `list_sessions` call.

**`wrapper_died` field (conditional):** same semantics as `list_sessions`. When crash detection fires for this session, the response includes `"wrapper_died": true`. Field is omitted otherwise. Lets Claude know whether to interpret the log tail as "ended on its own" or "abruptly cut off when peek died."

### `search_logs(id: string, pattern: string, context?: number, max_matches?: number)`

**Regex engine:** Go's `regexp` (RE2). Linear time, no catastrophic backtracking. Default case-sensitive — Claude can prefix `(?i)` for case-insensitive. The MCP tool description must mention the `(?i)` and `(?m)` flags.

**Parameters:**
- `id`: ULID, exact.
- `pattern`: RE2 regex, applied per line.
- `context`: lines of context before and after each match. Default 3, max 50.
- `max_matches`: hard cap. Default 50, max 200.

**Scope:** both `output.log.1` and `output.log`. Global line numbers.

**Truncation:** keep the **first** N matches (oldest). Server crashes typically have the root cause at the first matching log line; subsequent matches are follow-on noise. Returning newest-first throws away the answer in the common debugging case. The summary of the most recent match is preserved separately to capture "current state."

**Output format:** verbatim `grep -C` convention — `:` for matched lines, `-` for context lines, `--` between non-adjacent match groups. Drop any `>` markers. This is the format with the most LLM training-data exposure.

**Response:**

```json
{
  "matches": "1231-2026-05-03T14:23:01.230Z compiling src/foo.ts\n1232-2026-05-03T14:23:01.231Z compiling src/bar.ts\n1233:2026-05-03T14:23:01.232Z ERROR: failed to compile\n1234-2026-05-03T14:23:01.233Z   at line 42\n--\n1567:2026-05-03T14:24:55.501Z ERROR: ...\n",
  "match_count": 50,
  "total_matches": 200,
  "truncated": true,
  "last_match": {
    "line": 8923,
    "text": "ERROR: connection refused"
  }
}
```

`last_match` is included only when `truncated: true`.

**Adjacent matches:** when two matches are within `context` lines of each other, merge their context windows so we don't return duplicate context lines. Matches a real `grep -C`.

**Order:** chronological (oldest → newest within the result set).

## `peek list` Output

Human-friendly table, newest first, all sessions across all cwds:

```
ID         STATUS      STARTED               CMD              CWD
01H8XHS7Q  running     2026-05-03 14:23:01   npm run dev      ~/Code/myapp
01H7YGC2R  exited(0)   2026-05-03 12:10:33   cargo run        ~/Code/otherproj
01H6ABC12  exited(?)   2026-05-02 09:00:00   next dev         ~/Code/myapp
```

- ID truncated to 9 chars for display (full ULID is 26 chars and would dominate the row).
- `STATUS`: `running`, `exited(N)` for known exit code, `exited(?)` for null exit_code (wrapper-died case).
- `STARTED`: local time (not UTC). Humans read this; UTC stays in the on-disk log for correlation.
- `CMD` before `CWD` — matches `ps` and `docker ps`. Eyes scan commands faster than paths when looking for "which session is `npm run dev`."
- `CWD` shows `~` for home directory.
- `CMD` truncated to fit terminal width with `…`.

No flags in v1: no `--cwd`, no `--running-only`, no `--json`.

## `peek logs <id>`

`<id>` accepts unique ULID prefixes (matches what `peek list` displays). Ambiguous prefix → clear error to stderr, exit 1.

**Behavior:**
- Running session: stream new lines as they arrive (`tail -f` semantics). Exit on session exit or Ctrl+C.
- Exited session: dump the entire log to stdout, exit.

**Output format:** keep line numbers and timestamps in the output, formatted readably for humans:

```
   1231  14:23:01.230  compiling src/foo.ts
   1232  14:23:01.231  compiling src/bar.ts
   1233  14:23:01.232  ERROR: failed to compile
```

- Line number left-padded, fixed width.
- Timestamp abbreviated to `HH:MM:SS.mmm` in **local time**.
- Two spaces as column separator.

`peek logs` is a debugging tool, not a replay tool — colors and spinners are already gone, so we've already accepted it's a normalized post-hoc view. Line numbers let users correlate with what Claude references ("the error is at line 1233"). Raw bytes are still recoverable via `cat ~/.peek/sessions/<id>/output.log`.

## Repo Layout

```
peek/
├── cmd/peek/                       (main, CLI entry)
├── internal/
│   ├── wrapper/                    (pty, signals, log writing)
│   ├── ansi/                       (escape stripping + \r line discipline)
│   │   └── testdata/               (golden ANSI captures from real tools)
│   ├── store/                      (session dir layout, ring buffer eviction, scan)
│   └── mcp/                        (stdio server, three tool handlers)
├── .claude-plugin/
│   └── marketplace.json            (Claude Code marketplace entry)
├── .agents/plugins/
│   └── marketplace.json            (Codex marketplace entry)
├── plugins/peek/
│   ├── .claude-plugin/plugin.json
│   ├── .codex-plugin/plugin.json
│   └── .mcp.json                   (root-level MCP config for manual wiring)
├── .goreleaser.yaml
├── install.sh                      (curl-pipe-sh installer)
├── README.md
├── go.mod / go.sum
```

## Plugin Manifests

Inline `mcpServers` into each `plugin.json`. Two-line duplication is not worth designing a sharing mechanism around.

`plugins/peek/.claude-plugin/plugin.json`:

```json
{
  "name": "peek",
  "description": "Capture dev server logs and expose them to Claude via MCP.",
  "version": "0.1.0",
  "mcpServers": {
    "peek": {
      "command": "peek",
      "args": ["mcp"]
    }
  }
}
```

`plugins/peek/.codex-plugin/plugin.json`: analogous, conforming to Codex's plugin schema.

`plugins/peek/.mcp.json`: same `mcpServers` block, used by users wiring up manually via `claude mcp add` or editing config directly.

## Distribution

### Binary

Single Go binary built with **GoReleaser**. Targets: darwin/linux/windows × arm64/amd64. Published to GitHub Releases. (Specific GoReleaser config can be authored during implementation; no architectural pre-commitments.)

### Install script (`install.sh`)

Inspired by bun's installer.

- Default install location: `~/.local/bin/peek`. No sudo. Matches `uv`, `bun`, `deno`, `mise`.
- Detects whether `~/.local/bin` is on PATH; prints a helpful message with shell-specific instructions if not.
- No checksum verification in v1 — HTTPS to GitHub Releases is sufficient.
- Failure messages point at the GitHub Releases page for manual download.
- Windows: do not support from bash. Print `"see https://peek.sh/install#windows for Scoop install"` and exit non-zero.

### Package managers

- **Homebrew** tap: `brew install yourorg/tap/peek`.
- **Scoop** manifest for Windows.
- Direct: `curl -fsSL https://peek.sh/install | sh`.

### Binary discovery in `.mcp.json`

Plain `peek` command. Works when peek is on PATH (true after Homebrew, Scoop, or the install script). When not on PATH, the MCP spawn fails with a "command not found" error in Claude Code's MCP debug view.

The README troubleshooting section includes the exact error string ("command not found: peek" / Windows equivalent) so users searching for it land on the fix.

Two-step install (install binary, then install plugin) is the documented happy path.

## Cross-Platform

### PTY library
`github.com/aymanbagabas/go-pty`. Wraps `creack/pty` on Unix and ConPTY on Windows behind one API.

### Path handling
- `path/filepath` (not `path`) for all OS path work.
- `os.UserHomeDir()` for `~` substitution in `peek list` display.
- Session dir at `filepath.Join(os.UserHomeDir(), ".peek", "sessions")` — transparent across platforms.

### ANSI tests
Golden files captured from **both Unix and Windows** pty output. Windows ConPTY emits subtly different escape patterns — testing only on Unix is insufficient.

## Build Order

1. Wrapper that spawns child with pty, tees stdio, forwards signals, writes `meta.json` on start and exit. Test against `next dev`, `npm run dev`, `cargo run`, `python -m http.server`, `flask run`.
2. `peek list` and `peek logs` for human verification of the session store.
3. MCP server with `list_sessions` only. Wire into Claude Code via raw `claude mcp add` (or direct config edit) and confirm it sees sessions.
4. **Plugin manifests for Claude Code and Codex.** Install via `/plugin marketplace add` (and Codex equivalent). Confirm both clients can call `list_sessions` through the plugin install path. Catching plugin schema issues here — when only `list_sessions` exists — means the rest of MCP development happens on top of a working plugin install pipeline. The `.codex-plugin/plugin.json` schema in particular is worth verifying early since it has less ecosystem reference material to copy from.
5. `get_logs` and `search_logs`. The actual product. Now the plugin install already works, so each new tool surfaces through the existing install flow as it's added.
6. Per-cwd ring buffer eviction.
7. Cross-platform polish: Windows ConPTY, path handling, install script tested on macOS/Linux/Windows.
8. GoReleaser config, install script, Homebrew tap, Scoop manifest.
9. README with install steps for both clients.

## Definition of Done

A user on macOS, Linux, or Windows can:

1. Run a single install command for the binary.
2. Run a single command (or two for plugin install) to wire it into Claude Code or Codex.
3. Run `peek -- <their dev command>` and see logs as normal.
4. Ask Claude in another terminal "what's wrong with my dev server?" and have Claude pull logs without being told the working directory or session id.

These four must work for at least three different stacks: one Node, one Python, one other (Go, Rust, Ruby — pick one).

## Out of Scope (v1)

- Port detection.
- `git_root`, `project_name`, `package_name`, `resolved_cmd` fields.
- Script-runner resolution (`npm run X` → underlying command).
- Language/framework detection.
- `peek stop`, `peek restart`, or any process control.
- Filtering params on `list_sessions` besides cwd.
- Daemon, IPC socket, background process.
- Log rotation or size caps within a session beyond the 50/100 MB rule.
- Secrets redaction.
- Web UI, TUI dashboard, GUI of any kind.
- Skill bundled with the plugin (just MCP server).
- Submission to official marketplaces (do this post-launch with traction).
- Signal masking, SIGPIPE handling, nohup-style detachment.
- PID-reuse mitigation in crash detection.
- Symlink resolution in cwd matching.
- Case-insensitive cwd matching on macOS/Windows.
- `--cwd`, `--running-only`, `--json` flags on `peek list`.
- Configurable graceful-shutdown timeout.
- Configurable log rotation thresholds.
- Configurable ANSI handling.

## v2 Conversations Captured

Users will ask. Don't scope-creep.

- *"Can it restart my server?"* → No. That's the design constraint. If you want that, run the command yourself.
- *"Can it detect which framework I'm using?"* → No. Claude reads the logs and figures it out.
- *"Can I name my sessions?"* → Not yet. The id and cwd identify it.
- *"Can it forward to a hosted dashboard?"* → No. All data stays local.
- *"Can it filter logs server-side by level/timestamp/regex?"* → `search_logs` does regex. The rest is Claude's job.

## Implementation Notes

- Use `time.Now().UTC().Format(time.RFC3339Nano)` and trim to millisecond precision so timestamps in `meta.json` and on-disk log lines are byte-identical for string correlation.
- ULID generation uses `github.com/oklog/ulid/v2` with a monotonic entropy source seeded from `crypto/rand`.
- Raw-mode terminal toggle: `golang.org/x/term`, with `defer term.Restore(...)` immediately following the enable call.
- Ring buffer eviction is a single-pass scan that sorts matched sessions by ULID and `RemoveAll`s indexes 2 and beyond in one go. ENOENT-tolerant `meta.json` read helper for the scan path.
- `peek mcp` logs version and binary path to stderr at startup so Claude Code's `/mcp` debug view reveals install issues.

## Next Step

Move to `superpowers:writing-plans` to produce an implementation plan from this spec.
