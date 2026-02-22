# Manual Testing with Claude Code

Test hitch with a real Claude Code session. This uses the test project at `~/t/hitch-test/` which already has rules and a channel configured.

## Prerequisites

Hitch binary built and rules installed. Verify with:

```bash
~/dev/hitch/ht status
```

You should see 3 rules and 1 channel:
```
Channels: 1 configured
  desktop (desktop)

Rules: 3 total, 3 enabled
  [+] d07da5: on pre-bash -> deny if matches deny-list:destructive
  [+] fc482a: on stop -> log
  [+] 310ba6: on stop -> notify desktop if elapsed > 10s
```

If not, set it up from scratch:

```bash
mkdir -p ~/t/hitch-test
cd ~/t/hitch-test
~/dev/hitch/ht init
~/dev/hitch/ht channel add desktop
~/dev/hitch/ht rule add 'on pre-bash -> deny if matches deny-list:destructive'
~/dev/hitch/ht rule add 'on stop -> log'
~/dev/hitch/ht rule add 'on stop -> notify desktop if elapsed > 10s'
```

## Verify settings.json looks right

```bash
cat ~/t/hitch-test/.claude/settings.json
```

You should see hooks for `PreToolUse` (Bash matcher), `Stop` (two hooks), `SessionStart`, and `UserPromptSubmit`. All commands should point to `/home/user/dev/hitch/ht hook exec <id>`.

## Verify global settings are untouched

```bash
cat ~/.claude/settings.json
```

This should still have your `bd prime` hooks and `alwaysThinkingEnabled`. No `ht` entries.

---

## Test 1: Deny-List Blocking

**Start Claude Code in the test project:**

```bash
cd ~/t/hitch-test
claude
```

**Prompt:**

```
Run the command: echo "hello world"
```

**Expected:** Claude runs the command normally. The deny-list rule only blocks destructive patterns; `echo` is safe. Exit code 0 from hitch, Claude sees no interference.

**Prompt:**

```
Run this exact bash command: rm -rf /tmp/nonexistent-test-dir
```

**Expected:** Claude should be blocked. Hitch will return exit code 2 with `{"decision":"deny","reason":"blocked by hitch rule"}`. Claude should report that the command was blocked by a hook. The `rm -rf` pattern matches the deny-list.

**Prompt:**

```
Run: git push --force origin main
```

**Expected:** Blocked. Even though there's no git repo, the hook fires before execution. Hitch blocks based on the command string, not whether it would succeed.

**Things to watch for:**
- Does Claude report the block clearly?
- Does Claude try to work around the block, or does it respect it?
- Is there any noticeable latency from the hook?

---

## Test 2: Safe Commands Pass Through

**Prompt:**

```
Run: ls -la
```

**Expected:** Runs normally.

**Prompt:**

```
Run: npm --version
```

**Expected:** Runs normally.

**Prompt:**

```
Run: python3 -c "print('hello')"
```

**Expected:** Runs normally. None of these match deny-list patterns.

---

## Test 3: Stop Notification

This tests the `on stop -> notify desktop if elapsed > 10s` rule. You need to keep the session alive for at least 10 seconds.

**Prompt (give Claude something that takes a moment):**

```
List all files in this directory, then explain what each one does.
```

Wait for Claude to finish. Then either:
- Let Claude stop naturally (it will fire the Stop event)
- Or type `/stop` to end the session

**Expected:** If the session lasted more than 10 seconds from start, you should see a desktop toast notification saying something like "Hitch: Stop" with "Hook event: Stop". If the session was shorter than 10 seconds, no notification — the elapsed condition prevents it.

**Check the log after:**

```bash
~/dev/hitch/ht log --limit 5
```

You should see `Stop` events with action `notified:desktop` (if elapsed > 10s) or `condition-false` (if elapsed < 10s), plus any `PreToolUse` events from bash commands.

---

## Test 4: Event Logging

After running tests 1-3, check the event log:

```bash
~/dev/hitch/ht log
```

**Expected:** You should see entries for:
- `PreToolUse` events with action `denied` (for blocked commands) or `condition-false` (for allowed commands)
- `Stop` events with action `logged` (from the log rule) and `notified:desktop` or `condition-false` (from the notify rule)

Filter by event type:

```bash
~/dev/hitch/ht log --event PreToolUse
~/dev/hitch/ht log --event Stop
```

---

## Test 5: Mute

**Mute notifications:**

```bash
~/dev/hitch/ht mute 5m
~/dev/hitch/ht status
```

Status should show `Notifications: MUTED until <timestamp>`.

**Start another Claude session in the test dir and let it run > 10s, then stop.**

**Expected:** No desktop notification. The notify rule still fires but the adapter is suppressed by mute state.

**Unmute:**

```bash
~/dev/hitch/ht unmute
```

---

## Test 6: Rule Disable/Enable

**Disable the deny-list:**

```bash
~/dev/hitch/ht rule disable d07da5
cat ~/t/hitch-test/.claude/settings.json
```

The `PreToolUse` section should be gone from settings.json.

**Start Claude in the test dir and try a destructive command:**

```
Run: rm -rf /tmp/nonexistent-test-dir
```

**Expected:** The command runs (or fails because the dir doesn't exist, but it's not blocked by hitch). The deny-list rule is disabled.

**Re-enable:**

```bash
~/dev/hitch/ht rule enable d07da5
```

---

## Troubleshooting

**Hooks not firing at all:**
- Check that you started Claude in `~/t/hitch-test/`, not somewhere else. Project-scoped settings.json only applies in that directory.
- Check `cat ~/t/hitch-test/.claude/settings.json` — hooks should be there.
- Check that the `ht` binary path in settings.json is valid: the commands reference `/home/user/dev/hitch/ht`.

**Desktop notifications not appearing:**
- WSL may not route `notify-send` to Windows. Test with: `notify-send "test" "hello"` directly in the terminal.
- If `notify-send` doesn't work, the desktop adapter won't either. The hook still succeeds (exit 0) — it just can't display the notification.

**"channel not found" errors in hook execution:**
- The desktop channel must exist in the shared DB. Check with `~/dev/hitch/ht channel list`.
- If missing: `~/dev/hitch/ht channel add desktop`.

**Global settings modified:**
- This was a bug that has been fixed. If it happens again, check that you're running the latest built binary from `/home/user/dev/hitch/ht`.
- Restore global settings from the known-good state if needed.

---

## After Testing

Check the full state:

```bash
~/dev/hitch/ht status          # overview
~/dev/hitch/ht log             # all events
~/dev/hitch/ht log --since 1h  # recent events
~/dev/hitch/ht rule list       # rules and their scopes
~/dev/hitch/ht export          # rules as DSL
```

The test project persists at `~/t/hitch-test/`. To clean up hitch from it:

```bash
rm -rf ~/t/hitch-test/.hitch ~/t/hitch-test/.claude
```

To remove rules from the shared DB:

```bash
~/dev/hitch/ht rule remove d07da5
~/dev/hitch/ht rule remove fc482a
~/dev/hitch/ht rule remove 310ba6
```
