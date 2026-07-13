# AGENTS.md — read this first

You are an AI agent (or a human) picking up the **envkeep** project. This file
is your entrypoint. It exists so that **any session, any agent, can understand
what this project is, what phase it is in, and how we got here** — without
re-deriving the reasoning from scratch.

## Read order

1. **`docs/STATUS.md`** — where we are *right now*. Current phase, what is done,
   what is next, and a dated log of how we got here. Read this before doing
   anything.
2. **`docs/DECISIONS.md`** — *why* the project is shaped the way it is. Every
   non-obvious choice, the alternatives rejected, and the concrete trigger that
   would justify reconsidering it. If you are tempted to change a design choice,
   find its decision here first.
3. **`docs/DESIGN.md`** — *how* it works. Architecture, data layout on disk, the
   conflict state machine, the cache, the composition rule. The technical map.
4. **`docs/ROADMAP.md`** — *what* gets built and in what order. Phases, scope
   fences, and the triggers that unlock later phases.
5. **`README.md`** — the human-facing overview of the problem and the tool.

## Guardrails for anyone making changes

- **Do not expand scope past the current phase** (see `ROADMAP.md`). Features
  are deliberately fenced off with explicit triggers. If a trigger has not
  fired, the feature does not get built — that is a decision, not an oversight.
- **Every design choice has a recorded WHY.** If you change one, update
  `DECISIONS.md` with the new reasoning and move the old entry to a superseded
  state. Never silently reverse a decision — the history is the point.
- **Update `STATUS.md` at the end of any working session.** Append to its log
  so the next agent knows what changed and why.
- Decisions carry **reconsider-triggers**. Respect them: a fenced-off feature
  becomes in-scope only when its trigger fires, and the trigger is written down.

## One-paragraph summary of the project

`envkeep` is a Go CLI that keeps `.env` files in sync across the git worktrees
of a single repository. The git repo (its shared `git-common-dir`) is the source
of truth; a flat-file "vault" lives inside it. The tool tells you, per worktree,
whether the local `.env` is synced / ahead / behind / conflicting with the
vault, and lets you `push`/`pull`. A shell hook warns you on drift when you
`cd` into a worktree. It solves the daily pain of "I updated a var in one
worktree and forgot to propagate it to the others."
