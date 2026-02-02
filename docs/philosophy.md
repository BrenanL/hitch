# Philosophy & Vision

## What hitch is

Hitch is a hooks framework for AI coding agents. Today it targets Claude Code. Tomorrow it could target any agent system that exposes lifecycle events.

It has three faces:

1. **A tool** you use directly. `ht init`, add some rules, and your agent notifications, safety guards, and automations just work. You never think about JSON configs or shell scripts.

2. **A library** other tools can build on. The core — hook I/O parsing, state management, channel adapters, condition evaluation — is importable. An orchestration tool, a dashboard, a CI system can all use hitch's internals.

3. **A language** for describing agent behavior. The DSL (`on stop -> notify discord if elapsed > 30s`) is a declarative way to express what should happen at every point in an agent's lifecycle. It's not a programming language — it's a behavior specification.

## Design principles

### Invisible infrastructure

You should forget hitch is there. It installs once, configures in one command, and runs silently in the background. No daemons to manage, no servers to start, no config files to babysit. Like plumbing — you only notice it when it breaks.

Inspired by [Beads](https://github.com/steveyegge/beads): install globally, init per-project, invisible infrastructure underneath.

### Progressive disclosure

The simplest use case should take 30 seconds:

```bash
ht init --global
ht channel add ntfy my-alerts
ht rule add 'on stop -> notify ntfy'
```

The most complex use case should still be possible:

```
on stop -> run hook:custom-validation -> summarize -> notify slack if elapsed > 5m and tests-pass
```

Simple by default. Powerful when you need it.

### Agent-friendly by design

A large part of hitch's value comes from AI agents setting it up for humans. When someone says "set up hitch to text me when you're done and block any rm -rf commands," the agent should be able to:

1. Write custom hook scripts to `.hitch/hooks/`
2. Register rules via `ht rule add`
3. Configure channels via `ht channel add`

This means: clear CLI interface, predictable file locations, well-documented manifest format, and a custom hooks directory where agents can drop scripts without ceremony.

### Composable, not monolithic

Every piece should work independently:

- Channel adapters can send messages without the DSL
- The DSL parser can generate configs without the CLI
- The state manager can track events without notifications
- Custom hook scripts can run without knowing about hitch's internals

Other tools should be able to import the pieces they need.

### Not locked to Claude Code

Claude Code is the first target because it has the richest hooks system today. But the core concepts — events, conditions, actions, channels — are agent-agnostic. The architecture separates "what happened" (the event) from "what to do about it" (the action) from "how to deliver it" (the channel).

When other agent systems expose lifecycle hooks, hitch should be able to target them with minimal changes.

## Positioning

### Today (v1)

"A tool that makes Claude Code hooks effortless."

Concrete, useful, solves real problems. The deny-list-for-skip-permissions angle is the strongest immediate value prop. The notification system is the most relatable. The DSL is the differentiator.

### Tomorrow (v2)

"A framework for agent automation."

Multi-channel routing, hook packages, custom hook marketplace, integration with CI/CD, cross-session state. Hitch becomes the standard way to add behaviors to your agent workflows.

### Eventually (v3)

"A language for describing agent behaviors."

The DSL becomes the primary interface. You describe your entire agent automation stack in a `.hitch` file. Other tools read it. Agents write it. It's the configuration layer between humans and AI systems.

## The educational angle

Every hook idea is a potential article, tutorial, or social post:

- "I built a framework that lets you describe Claude Code hooks in plain English"
- "How I use a deny-list to safely run Claude Code without permission prompts"
- "My agent texts me when it's done — here's the one-line setup"
- "I made Claude prove its tests pass before it's allowed to stop"
- "This tool scans every file my AI writes for leaked secrets"
- "How I run 5 agents across 5 projects and track them all from one dashboard"

Hook packages are shareable content. The DSL rules are copy-pasteable. Every feature is a story.

## The long-term dream

Hitch is a building block for something larger: a system that oversees multiple agents across multiple projects, tracks their work, manages task queues, enforces quality standards, and gives you a single pane of glass for everything your AI agents are doing.

That system is not hitch. But hitch is the automation layer it builds on. The SQLite database, the event log, the channel adapters, the state management — all of that feeds upward into an orchestration layer.

For now, hitch is a standalone tool that solves real problems today. The architecture just happens to be ready for what comes next.

## Influences

### Beads

[Beads](https://github.com/steveyegge/beads) showed that a CLI tool can feel effortless: install once globally, one-command project setup, invisible infrastructure. Their use of Go for single-binary distribution, SQLite for state, and hash-based IDs for multi-agent safety directly informed hitch's architecture.

### OpenClaw

[OpenClaw](https://github.com/openclaw/openclaw) showed the adapter pattern for messaging channels: one unified interface, per-platform implementations using established libraries. Their gateway architecture is more than we need, but the connector model is exactly right for our channel adapters.

### Claude Code's hooks system

Claude Code's hooks are remarkably well-designed: 12 lifecycle events, rich JSON I/O, three hook types (command, prompt, agent), async support, matchers, decision control. Hitch wouldn't exist without this foundation. The gap hitch fills is the UX layer on top — making hooks declarative instead of manual.
