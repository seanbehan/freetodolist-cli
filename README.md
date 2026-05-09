# freetodolist-cli

A small, scriptable command-line client for [FreeTodoList](https://freetodolist.com).
Single static binary, OAuth login, and a `--json` flag on every command.

```
$ freetodolist login
Opening browser to log in…
Logged in as you@example.com

$ freetodolist overdue
3 overdue items
DUE         DAYS  LIST       BODY
2026-05-02   7    Side proj  Ship the auth fix
2026-05-04   5    Home       Schedule dentist
2026-05-06   3    Work       Reply to design review
```

## Install

Download the binary for your platform from the
[releases page](https://github.com/seanbehan/freetodolist-cli/releases),
extract, and put it on your `PATH`:

```bash
# macOS / Linux
tar -xzf freetodolist-v0.1.0-<os>-<arch>.tar.gz
sudo mv freetodolist-v0.1.0-<os>-<arch>/freetodolist /usr/local/bin/

# Windows: unzip and place freetodolist.exe on your PATH.
```

Or build from source (requires Go 1.21+):

```bash
git clone https://github.com/seanbehan/freetodolist-cli
cd freetodolist-cli
go build -o /usr/local/bin/freetodolist
```

## Login

```bash
freetodolist login
```

This walks an OAuth 2.1 + PKCE flow against `freetodolist.com`: it opens
your browser, captures the callback on a loopback port, and saves a
Bearer token to `~/.config/freetodolist/credentials.json` (mode 0600).
The same token authenticates the JSON API.

To target a different deployment (e.g. local dev), pass `--base-url` or
set `FREETODOLIST_BASE_URL`.

## Commands

```
login / logout / whoami   OAuth flow + identity check

lists list                list your todo lists
lists show <list-uid>     list metadata + items + tabs

items list <list-uid>     items in a list (filter by tab/state/due)
items create <list-uid> --body=<text> [--due=<RFC3339>]
items show <item-uid>
items update <item-uid> [--body=...] [--complete=true|false] [--due=...]
items delete <item-uid>

tabs list <list-uid>
tabs create <list-uid> --name=<name>
tabs update <list-uid> <slug> --name=<name>
tabs delete <list-uid> <slug>
tabs sort   <list-uid> <slug1,slug2,...>
tabs assign <list-uid> --items=<id,id,...> [--tab=<slug>]

dashboard                 account-wide stats
overdue                   overdue items across all lists
due                       items due in the next 30 days
shared <token>            read a publicly shared list (no auth)
```

`--json` works on every command. Pipe to `jq`, `grep`, `fzf`, `xargs` —
anything you'd normally reach for in a Unix toolbelt.

## Configuration

| Setting   | Flag                | Env                      | Default                     |
| --------- | ------------------- | ------------------------ | --------------------------- |
| API token | `--token <token>`   | `FREETODOLIST_TOKEN`     | from credentials file       |
| Base URL  | `--base-url <url>`  | `FREETODOLIST_BASE_URL`  | `https://freetodolist.com`  |
| Output    | `--json`            | —                        | human-readable tables       |

Token resolution order: flag > env > saved credentials.

## Use with AI agents

The CLI is built to be driven by AI coding agents (Claude Code, Cursor,
Aider, Codex CLI). FreeTodoList publishes a SKILL.md for it that any
agent can fetch and follow:

<https://freetodolist.com/.well-known/agent-skills/freetodolist-cli/SKILL.md>

If your agent supports MCP and you'd rather use that, FreeTodoList also
exposes an MCP server: <https://freetodolist.com/mcp_docs>.

## Releasing

```bash
VERSION=v0.1.0 ./build.sh
```

Produces `dist/freetodolist-<version>-<os>-<arch>.{tar.gz,zip}` plus a
`SHA256SUMS` file. Upload to a GitHub Release.

## License

MIT — see [LICENSE](LICENSE).
