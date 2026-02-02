# Hook Ideas — What's Possible with Claude Code Hooks

This document catalogs practical, creative, and ambitious things you can build with Claude Code hooks. Each idea includes which hook events it uses, what it does, and why you'd want it.

These are organized into "hook packages" — themed bundles that solve a category of problem. Each package could be installed independently, shared as a config, or described in plain English through the DSL.

---

## Package: Notifier

**The core MVP.** Get notified through any channel when Claude needs you or finishes work.

### 1. Stop Notification
- **Events:** `Stop`
- **What:** Send a push notification / text / Discord message when Claude finishes responding.
- **Why:** Walk away from your machine, do other work, get pinged when it's done.
- **Nuance:** Only notify if Claude ran for >N seconds (skip notifications for quick responses). Check if the terminal is focused — no notification needed if you're watching. Check `stop_hook_active` to avoid double-notifying on continuations.
- **DSL:** `on stop -> notify discord if elapsed > 30s`

### 2. Permission Alert
- **Events:** `Notification` (matcher: `permission_prompt`)
- **What:** Send an alert when Claude is blocked waiting for permission approval.
- **Why:** If you're away and Claude hits a permission wall, you want to know immediately so it's not sitting idle.
- **DSL:** `on notification:permission -> notify sms "Claude needs permission"`

### 3. Idle Alert
- **Events:** `Notification` (matcher: `idle_prompt`)
- **What:** Notify when Claude has been waiting for your input.
- **Why:** You might not realize Claude finished and is waiting for your next instruction.
- **DSL:** `on notification:idle -> notify pushover`

### 4. Error Escalation
- **Events:** `PostToolUseFailure`
- **What:** When a tool fails (test suite crashes, command errors), send an alert with the error details.
- **Why:** If you're away, you want to know that something went wrong, not just that Claude stopped.
- **DSL:** `on tool-failure -> notify slack with error`

### 5. Progress Heartbeats
- **Events:** `PostToolUse` (async)
- **What:** Periodically send updates about what Claude is doing: "Editing auth.ts", "Running test suite", "Reading config files". Throttled to every N minutes.
- **Why:** When you've kicked off a long task, knowing that progress is happening (and what kind) gives peace of mind without watching the terminal.
- **DSL:** `on post-tool -> heartbeat discord every 5m`

### 6. Smart Routing
- **Events:** All notification events
- **What:** Route notifications to different channels based on time of day, urgency, or context. Slack during work hours, SMS after hours. Permission prompts always go to SMS, idle notifications go to email digest.
- **Why:** Avoid notification fatigue. Not everything deserves an SMS.
- **DSL:** `on stop -> notify slack if 9am-5pm, notify sms if not 9am-5pm`

### 7. Focus Mode Detection
- **Events:** `Stop`, `Notification`
- **What:** Detect whether the user is actively at their terminal (terminal focused, recent keystrokes, mouse activity) and suppress notifications when they are. On macOS this is possible via AppleScript; on Linux via xdotool or D-Bus; on WSL via PowerShell interop.
- **Why:** The key insight from the original request — you don't want a text when you're pair-programming, but you do want one when you've walked away.
- **DSL:** `on stop -> notify sms if away`

---

## Package: Safety Guard

**A deny-list for `--dangerously-skip-permissions`.** Block destructive commands while still enjoying frictionless tool use.

### 8. Destructive Command Blocker
- **Events:** `PreToolUse` (matcher: `Bash`)
- **What:** Maintain a deny-list of dangerous command patterns (`rm -rf /`, `DROP DATABASE`, `git push --force origin main`, `chmod -R 777 /`, `mkfs`, `dd if=`, `:(){ :|:& };:`) and block them before execution. Claude receives the reason and can adjust.
- **Why:** `--dangerously-skip-permissions` has no built-in deny-list. This is the workaround. You get speed without recklessness.
- **DSL:** `on pre-bash -> deny if matches deny-list:destructive`

### 9. Protected File Guard
- **Events:** `PreToolUse` (matcher: `Edit|Write`)
- **What:** Prevent edits to sensitive files: `.env`, `.git/`, `package-lock.json`, credentials, SSH keys, etc.
- **Why:** Even with skip-permissions, some files should never be touched by an agent.
- **DSL:** `on pre-edit -> deny if file matches protect-list`

### 10. Protected Branch Guard
- **Events:** `PreToolUse` (matcher: `Bash`)
- **What:** Block `git push` to main/master/production branches. Allow pushes to feature branches.
- **Why:** An agent shouldn't be able to push directly to production, even accidentally.
- **DSL:** `on pre-bash -> deny if git-push to main|master|production`

### 11. Secret Scanner
- **Events:** `PostToolUse` (matcher: `Write|Edit`)
- **What:** After Claude writes or edits a file, scan it for patterns that look like API keys, tokens, passwords, or other secrets. Alert immediately if found.
- **Why:** Prevent accidental secret commits. Catches the problem at write-time, before it ever reaches git.
- **DSL:** `on post-edit -> scan for secrets -> alert if found`

### 12. Dependency Audit
- **Events:** `PreToolUse` (matcher: `Bash`)
- **What:** Intercept `npm install`, `pip install`, `cargo add` commands. Check the package name against known vulnerability databases or a curated blocklist. Block installation of compromised packages.
- **Why:** Supply chain attacks are real. An agent shouldn't be able to install a typosquatted malicious package without scrutiny.
- **DSL:** `on pre-bash -> audit if package-install`

### 13. Network Request Guard
- **Events:** `PreToolUse` (matcher: `Bash|WebFetch`)
- **What:** Block or flag outbound network requests to unexpected domains. Maintain an allowlist of acceptable endpoints.
- **Why:** Prevent data exfiltration or unexpected API calls, especially when running with skip-permissions.
- **DSL:** `on pre-bash -> deny if curl to unknown-domain`

---

## Package: Quality Gate

**Force Claude to verify its own work before declaring done.**

### 14. Test Gate
- **Events:** `Stop` (agent-type hook)
- **What:** When Claude says it's done, spawn an agent that runs the test suite. If tests fail, block the stop and tell Claude to fix the failures.
- **Why:** Claude often says "I've made the changes" without running tests. This ensures tests pass before the human sees a "done" notification.
- **DSL:** `on stop -> require tests-pass`

### 15. Lint Gate
- **Events:** `Stop` or `PostToolUse` (matcher: `Write|Edit`)
- **What:** Run the project's linter after edits or before stopping. Feed lint errors back to Claude.
- **Why:** Keeps code quality consistent without relying on Claude to remember to lint.
- **DSL:** `on post-edit -> run lint`

### 16. Type Check Gate
- **Events:** `Stop` (agent-type)
- **What:** Run `tsc --noEmit` or equivalent type checker before allowing Claude to stop.
- **Why:** Type errors are easy to introduce and Claude doesn't always catch them.
- **DSL:** `on stop -> require typecheck-pass`

### 17. Coverage Gate
- **Events:** `Stop` (agent-type)
- **What:** Check that test coverage hasn't dropped below a threshold. Block Claude from stopping if coverage regressed.
- **Why:** Prevent shipping code without adequate test coverage.
- **DSL:** `on stop -> require coverage >= 80%`

### 18. Build Gate
- **Events:** `Stop`
- **What:** Run the build process. If it fails, block the stop and tell Claude to fix it.
- **Why:** "It compiles" is a minimum bar.
- **DSL:** `on stop -> require build-pass`

### 19. Completion Verifier
- **Events:** `Stop` (prompt-type hook)
- **What:** Use a prompt hook that reads the transcript and evaluates: "Did Claude actually complete all tasks the user asked for?" If not, block with a reason.
- **Why:** Claude sometimes stops prematurely, especially on multi-part requests.
- **DSL:** `on stop -> verify all-tasks-complete`

---

## Package: Automation

**Make things happen automatically when Claude does things.**

### 20. Auto-Format
- **Events:** `PostToolUse` (matcher: `Write|Edit`)
- **What:** Run Prettier/Black/rustfmt on every file Claude edits.
- **Why:** Consistent formatting without relying on Claude to format correctly.
- **DSL:** `on post-edit -> run prettier`

### 21. Auto-Commit
- **Events:** `Stop`
- **What:** When Claude finishes a task, automatically stage and commit changes with a generated message. Optionally only if tests pass.
- **Why:** Create a clean commit history of agent work without manual intervention.
- **DSL:** `on stop -> auto-commit if tests-pass`

### 22. Auto-PR
- **Events:** `Stop`
- **What:** When Claude finishes, create a GitHub PR with a summary of changes, diff stats, and what was accomplished (parsed from transcript).
- **Why:** Go from "agent finished" to "PR ready for review" with zero manual steps.
- **DSL:** `on stop -> create-pr`

### 23. Auto-Deploy
- **Events:** `Stop` (chained)
- **What:** Run tests -> build -> deploy to staging, all triggered by Claude finishing. With notifications at each step.
- **Why:** Full CI/CD pipeline triggered by agent completion.
- **DSL:** `on stop -> test -> build -> deploy staging -> notify team`

### 24. Task Queue Consumer
- **Events:** `Stop`
- **What:** When Claude finishes, read the next task from a queue (file, SQLite, Redis, API) and feed it as the next instruction by blocking the stop with a reason.
- **Why:** Chain multiple tasks without human intervention. Let Claude work through a backlog overnight.
- **DSL:** `on stop -> next-task from queue`

### 25. Auto-Changelog
- **Events:** `PostToolUse` (matcher: `Write|Edit`, async)
- **What:** Maintain a running changelog entry for the current session, noting every file modified and a brief description of the change.
- **Why:** Automatic documentation of what changed and why.
- **DSL:** `on post-edit -> append changelog`

---

## Package: Observability

**See what your agents are doing, track costs, build dashboards.**

### 26. Session Recap
- **Events:** `Stop` or `SessionEnd`
- **What:** Parse the transcript and generate a human-readable summary of what was accomplished. Send it via notification or save to a file.
- **Why:** Instead of "Claude is done", get "Claude refactored the auth module, added 3 tests, fixed the login bug, and updated the README."
- **DSL:** `on stop -> summarize -> notify discord`

### 27. Diff Digest
- **Events:** `SessionEnd`
- **What:** Compute `git diff` since session start and send a formatted summary of all file changes.
- **Why:** Quick review of everything the agent touched without opening your editor.
- **DSL:** `on session-end -> send diff-digest`

### 28. Cost Tracker
- **Events:** `SessionEnd`, `Stop`
- **What:** Track token usage and estimated cost per session. Maintain running totals. Alert when approaching daily/weekly/monthly budgets.
- **Why:** Agents can burn through tokens fast. Know what you're spending.
- **DSL:** `on session-end -> track cost -> alert if budget > 80%`

### 29. Command Logger
- **Events:** `PostToolUse` (matcher: `Bash`)
- **What:** Log every shell command Claude executes to a structured audit trail. Include timestamps, exit codes, and truncated output.
- **Why:** Forensics and accountability. Know exactly what an agent did.
- **DSL:** `on post-bash -> log command`

### 30. Activity Dashboard Feed
- **Events:** All events
- **What:** Emit structured events (JSON lines or webhook) from every hook to feed a real-time dashboard showing agent activity across all projects.
- **Why:** When you have multiple agents running across projects, you want a single pane of glass.
- **DSL:** `on all -> emit to dashboard`

### 31. Time Tracker
- **Events:** `SessionStart`, `SessionEnd`
- **What:** Log session duration to Toggl, Clockify, or a local timesheet file. Tag by project.
- **Why:** Track how much agent time is spent on each project.
- **DSL:** `on session-start -> start timer; on session-end -> stop timer`

### 32. Standup Generator
- **Events:** `SessionEnd`
- **What:** Append a session summary to a daily standup file. At the end of the day, you have a ready-made standup report.
- **Why:** Never write a standup again. Let your agents document their own work.
- **DSL:** `on session-end -> append standup`

---

## Package: Context & Knowledge

**Preserve and share knowledge across sessions, compactions, and teams.**

### 33. Compaction Saver
- **Events:** `PreCompact`
- **What:** Before context compaction, extract key decisions, patterns, and learnings from the transcript and save them to an external knowledge base or file.
- **Why:** Compaction loses detail. Save the important bits before they're summarized away.
- **DSL:** `on pre-compact -> save knowledge`

### 34. Cross-Session Memory
- **Events:** `SessionStart`
- **What:** Load relevant context from previous sessions based on the current project, branch, or recent changes. Could pull from a local DB or shared knowledge store.
- **Why:** Claude starts fresh each session. This gives it institutional memory.
- **DSL:** `on session-start -> load memory for project`

### 35. Team Context Broadcaster
- **Events:** `PostToolUse` (matcher: `Write|Edit`, async)
- **What:** When Claude makes architectural decisions or significant changes, broadcast a summary to a team channel.
- **Why:** Keep the team aware of AI-driven changes as they happen.
- **DSL:** `on post-edit -> broadcast if significant`

### 36. CLAUDE.md Auto-Enricher
- **Events:** `SessionEnd`
- **What:** Parse the session transcript for newly discovered project conventions, gotchas, or patterns. Append them to CLAUDE.md for future sessions.
- **Why:** The project's institutional knowledge grows automatically.
- **DSL:** `on session-end -> enrich claude.md`

### 37. Conflict Radar
- **Events:** `PreToolUse` (matcher: `Edit|Write`)
- **What:** Before Claude edits a file, check if that file was recently modified by a teammate (via git). Warn Claude or the user about potential conflicts.
- **Why:** Avoid merge conflicts and stepping on each other's work.
- **DSL:** `on pre-edit -> warn if file recently-changed by others`

---

## Package: Agent Orchestration

**Tools for agents to instrument themselves or coordinate with other agents.**

### 38. Self-Instrumentation
- **What:** Give an agent access to the hook CLI so it can set up its own hooks before starting a long task. "Before I begin this 30-minute refactor, let me set up a notification for when I'm done and a test gate to verify my work."
- **Why:** Agents become self-aware about their operational needs.

### 39. Agent-to-Agent Signaling
- **Events:** `SubagentStop`
- **What:** When a subagent finishes, update a shared state file (SQLite) so other agents/sessions can see what was completed and pick up dependent work.
- **Why:** Multi-agent workflows need coordination.

### 40. Error Recovery Pipeline
- **Events:** `PostToolUseFailure`
- **What:** When a tool fails repeatedly, escalate through a chain: retry -> different approach -> notify human. Track failure counts in state.
- **Why:** Don't let an agent spin on a broken approach. Escalate intelligently.

### 41. Environment Bootstrapper
- **Events:** `SessionStart`
- **What:** Set up Docker containers, install dependencies, configure environment variables, and prepare the workspace based on the project type.
- **Why:** "Just works" environment setup every time an agent starts.

### 42. Scheduled Task Injector
- **Events:** `SessionStart`
- **What:** Check a cron-like schedule or task queue and inject today's/this hour's tasks as context for the session.
- **Why:** Agents can work on scheduled maintenance, regular checks, or queued work without human prompting.

---

## Package: Developer Experience

**Quality of life improvements that make working with agents more pleasant.**

### 43. Sound Effects
- **Events:** `Stop`, `PostToolUseFailure`
- **What:** Play a sound when Claude finishes (success sound) or when an error occurs (error sound). Different sounds for different events.
- **Why:** Simple audio feedback without looking at the screen. Surprisingly satisfying.
- **Note:** On WSL, requires PowerShell interop to play sounds through Windows.
- **DSL:** `on stop -> play sound:success; on tool-failure -> play sound:error`

### 44. Status Bar Integration
- **Events:** All events
- **What:** Update tmux status bar, polybar, waybar, or other system bars with current agent activity: "Claude: editing auth.ts" or "Claude: idle".
- **Why:** Ambient awareness of what your agent is doing.
- **DSL:** `on all -> update statusbar`

### 45. Terminal Title Updates
- **Events:** `PostToolUse`, `Stop`
- **What:** Set the terminal title/tab name to reflect current agent activity. "Claude: testing..." / "Claude: done".
- **Why:** When you have multiple terminals, know which ones have active agents at a glance.
- **DSL:** `on post-tool -> set terminal-title`

### 46. Git Branch Namer
- **Events:** `SessionStart`
- **What:** If on the default branch, automatically create and switch to a descriptive feature branch based on the first prompt.
- **Why:** Never forget to branch before starting work.

---

## The Educational & Content Angle

Each of these packages is a potential article, tutorial, or social post:

- "I built a framework that lets you describe Claude Code hooks in plain English"
- "How I use a deny-list to safely run Claude Code without permission prompts"
- "My agent texts me when it's done — here's the one-line setup"
- "I made Claude prove its tests pass before it's allowed to stop"
- "This tool scans every file my AI writes for leaked secrets"
- "How I run 5 agents across 5 projects and track them all from one dashboard"
- "My agent writes its own standup reports"
- "I gave my agent a task queue and let it work through 20 issues overnight"

The "hook packages" concept is especially shareable — people can mix and match the behaviors they want, and the plain-English DSL makes it feel accessible even to non-technical users.

### Hook Package Distribution

Packages could be shared as:
- Single config files that can be imported (`ht package enable notifier`)
- Git repos with hook scripts + config
- Community marketplace entries
- One-liner install commands in blog posts

### The "Describe What You Want" Pitch

The most compelling marketing angle: "Instead of writing JSON configs and shell scripts, just tell the tool what you want in plain English and it generates the hooks for you." This could be the headline for every piece of content.
