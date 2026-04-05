# hitch

A hooks framework for AI coding agents. Describe what you want in plain English, and hitch turns it into working hooks.

```
on stop -> notify discord if elapsed > 30s
on pre-bash -> deny if matches "rm -rf"
on stop -> require tests-pass
on post-edit -> run "npm test" async
```

## What it does

Hitch sits between you and your AI agent (Claude Code today, others later). It lets you declare behaviors — notifications, safety guards, quality gates, automations — using a simple DSL, and generates the hook configurations and scripts that make them work.

**Notify me when it's done.** Get a text, Discord message, or push notification when your agent finishes a task. Only when you've been away for 30+ seconds — not when you're watching.

**Block dangerous commands.** Run with `--dangerously-skip-permissions` safely. Hitch adds a deny-list that blocks destructive patterns (`rm -rf /`, `DROP DATABASE`, `git push --force main`) while letting everything else through.

**Force quality gates.** Make your agent prove tests pass before it's allowed to stop. Auto-format every file it edits. Run linters after every change.

**Automate workflows.** Auto-commit when done. Create a PR. Run your deploy pipeline. Chain tasks from a queue.

**Describe it in plain English.** The DSL compiles to hook scripts and JSON configuration. You never edit settings.json manually.

## Quick start

```bash
# Install
go install github.com/BrenanL/hitch/cmd/ht@latest

# Or build from source
git clone https://github.com/BrenanL/hitch.git
cd hitch && go build -o ht ./cmd/ht

# Initialize globally
ht init --global

# Add a notification channel
./ht channel add ntfy my-alerts

# Add your first rule
./ht rule add 'on stop -> notify ntfy if elapsed > 30s'

# Done. Next time Claude Code finishes a long task, you'll get a push notification.
```

## Per-project setup

```bash
cd my-project
ht init
ht rule add 'on pre-bash -> deny if matches deny-list:destructive'
ht rule add 'on stop -> require tests-pass'
```

## Architecture

```
 You write          hitch generates       Claude Code reads
+-----------+      +----------------+     +------------------+
| DSL rules | ---> | Hook scripts + | --> | settings.json    |
|           |      | JSON config    |     | hooks entries     |
+-----------+      +----------------+     +------------------+
                          |
                   +------+------+
                   | SQLite DB   |  State, events, sessions
                   +------+------+
                          |
                   +------+------+
                   | Adapters    |  ntfy, Discord, Slack, SMS...
                   +-------------+
```

Hitch is a single Go binary. No runtime dependencies. Install it once, use it everywhere.

## API Logging Proxy

Hitch includes a transparent HTTP proxy that sits between Claude Code and the Anthropic API, logging every request and response with full token counts, cost tracking, latency, and automatic bug detection.

```bash
ht proxy start           # Start the proxy
ht proxy sessions        # List sessions with costs
ht proxy session <id>    # Full transaction list for a session
ht proxy analyze <id>    # Content breakdown: system, tools, messages
ht proxy stats --today   # Today's aggregate stats
```

See `internal/proxy/README.md` for full proxy documentation.

## Notification channels

| Channel | Setup complexity | What you need |
|---|---|---|
| ntfy.sh | None | Just pick a topic name |
| Discord | Trivial | A webhook URL |
| Slack | Trivial | A webhook URL |
| Desktop | None | OS-native (macOS/Linux/WSL) |
| Telegram | Low | Bot token + chat ID |
| Pushover | Low | App token + user key |
| Email | Medium | SMTP or SendGrid credentials |
| Twilio SMS | Medium | Account SID + auth token |

## Hook packages

Pre-built bundles of rules for common needs:

- **notifier** — Stop notifications, permission alerts, progress heartbeats
- **safety** — Destructive command blocker, protected files, secret scanner
- **quality** — Test gate, lint gate, type check gate
- **observer** — Session recaps, diff digests, cost tracking, command logging

```bash
ht package enable notifier
ht package enable safety
```

## Documentation

- [Agent & Development Guide](docs/agent-guide.md) — Conventions, test patterns, build commands
- [Claude Code Hooks Overview](docs/hooks-overview.md) — Quick reference for the hooks API
- [Design Review](docs/analysis/design-review.md) — Post-MVP review: what works, what's missing

## License

MIT
