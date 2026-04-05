# Real End-to-End Test Results

Date: 2026-02-02
Binary: `ht` built from `/home/user/dev/hitch/`
Test project: `~/t/hitch-test/`

## Setup

Created an isolated project directory. Ran `ht init` (project scope — no `--global`). This created:
- `~/t/hitch-test/.hitch/` — project hitch directory
- `~/t/hitch-test/.claude/settings.json` — project-level Claude settings with system hooks
- `~/.hitch/state.db` — shared SQLite database (created if not present)

Global `~/.claude/settings.json` was verified untouched after every operation.

## Rules Installed

```
ht rule add 'on pre-bash -> deny if matches deny-list:destructive'   → rule d07da5
ht rule add 'on stop -> log'                                          → rule fc482a
ht rule add 'on stop -> notify desktop if elapsed > 10s'              → rule 310ba6
```

Channel: `desktop` adapter (uses `notify-send` on Linux/WSL).

## Deny-List Test Results

Simulated Claude Code hook invocations by piping JSON to `ht hook exec d07da5`:

| Command | Result | Exit Code |
|---|---|---|
| `rm -rf /` | BLOCKED | 2 |
| `git push --force origin main` | BLOCKED | 2 |
| `DROP DATABASE users` | BLOCKED | 2 |
| `chmod -R 777 /` | BLOCKED | 2 |
| `sudo rm -rf /var` | BLOCKED | 2 |
| `dd if=/dev/zero of=/dev/sda` | BLOCKED | 2 |
| `ls -la` | ALLOWED | 0 |
| `npm test` | ALLOWED | 0 |
| `python script.py` | ALLOWED | 0 |
| `echo hello` | ALLOWED | 0 |

Blocked output: `{"decision":"deny","reason":"blocked by hitch rule"}`
Allowed output: `{}`

## Elapsed Condition + Notification Test

1. Sent `SessionStart` event for session `hitch-test-1` → recorded session start time in KV store
2. Immediately sent `Stop` event to rule 310ba6 → `condition-false` (elapsed < 10s), no notification
3. Waited 12 seconds, sent `Stop` again → `notified:desktop` (elapsed > 10s), desktop toast notification appeared on screen

Event log confirmed:
```
2026-02-02T17:23:39  Stop  310ba6  518ms  notified:desktop
2026-02-02T17:23:19  Stop  310ba6  0ms    condition-false
```

The 518ms duration for the notification delivery is the round-trip through `notify-send`.

## System Hooks

- `session-start`: Recorded session in DB, set `session_start:<session_id>` KV entry. Exit 0.
- `user-prompt`: Not explicitly tested but installed in settings.json.

## Rule Lifecycle

- `rule disable d07da5` → removed PreToolUse hook from settings.json, rule shows `[-]`
- `rule enable d07da5` → re-added hook to settings.json, rule shows `[+]`
- Settings.json correctly reflected state after each operation

## Other Commands Tested

| Command | Result |
|---|---|
| `ht status` | Showed 1 channel, 3 rules, mute state, recent events |
| `ht log --event PreToolUse` | Filtered correctly |
| `ht log --since 1m` | Filtered correctly |
| `ht log --limit 2` | Limited correctly |
| `ht mute 5m` | Set mute until timestamp |
| `ht unmute` | Cleared mute |
| `ht deny-list list` | Showed "destructive" with 20 patterns |
| `ht deny-list show destructive` | Listed all 20 patterns |
| `ht package list` | Showed 4 packages |
| `ht package show safety` | Showed 3 rules in safety package |
| `ht export` | Output valid .hitch DSL |
| `ht sync --dry-run` | Previewed 5 entries without writing |
| `ht notify desktop "Hitch is working!"` | Desktop toast appeared |
| `ht channel list` | Showed desktop channel with last-used timestamp |

## Bugs Found and Fixed During Testing

### 1. `resolveAdapter` didn't parse channel config JSON

**File:** `internal/cli/config.go`
**Impact:** Notifications during hook execution would create adapters with empty config, causing failures for any adapter needing configuration (ntfy topic, webhook URLs).
**Fix:** Added `json.Unmarshal([]byte(ch.Config), &config)` — one line.

### 2. `runSyncInternal` always synced global scope

**File:** `internal/cli/rule.go`
**Impact:** Any project-scoped `rule add` would write system hooks to `~/.claude/settings.json`, modifying global settings as a side effect.
**Fix:** Only sync global scope if global-scoped rules exist AND a global manifest is already present.

## Global Settings Isolation

Verified after every mutating operation (`init`, `rule add` x3, `rule disable`, `rule enable`, `sync`) that `~/.claude/settings.json` was byte-identical to its original state. The sync fix works correctly.

## Generated settings.json

Final project settings.json at `~/t/hitch-test/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/dev/hitch/ht hook exec d07da5 # ht:rule-d07da5"
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/dev/hitch/ht hook exec system:session-start # ht:system:session-start"
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/dev/hitch/ht hook exec fc482a # ht:rule-fc482a"
          },
          {
            "type": "command",
            "command": "/home/user/dev/hitch/ht hook exec 310ba6 # ht:rule-310ba6"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/dev/hitch/ht hook exec system:user-prompt # ht:system:user-prompt"
          }
        ]
      }
    ]
  }
}
```

## Conclusion

The MVP works end-to-end for the core use cases: deny-list blocking, event logging, elapsed-time conditions, desktop notifications, rule lifecycle management, and settings.json sync with project isolation. The two bugs found were in the adapter config path (would have broken real-world notifications) and the sync scope (would have modified global settings). Both fixed.
