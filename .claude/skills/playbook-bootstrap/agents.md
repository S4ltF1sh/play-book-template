# Agent hierarchy & model tiers (shared by all playbook skills)

The single source for who delegates to whom, which model each role runs on, and how results travel back. `playbook-bootstrap`, `playbook-setup-env` and `playbook-generate` all follow this file — change it here, not in the skills.

## Hierarchy

```
PM — the session (playbook-bootstrap, or whichever skill the user invoked directly)   Fable / Opus
├── setup coordinator — playbook-setup-env                                           opus (or sonnet), user-confirmed
│   ├── system-package worker                                                        haiku
│   └── one private-env worker per toolchain                                         haiku or sonnet, per sub-task
└── chapter writers — playbook-generate auto mode                                    opus / sonnet, see tiers
```

When the user invokes `playbook-setup-env` directly, the session is both PM and coordinator.

Each level owns its own verification and hands **one** compact report to its parent. A parent never supervises how a child works.

## Model tiers (actual versions resolve to the user's plan)

| Role | Model | Notes |
|------|-------|-------|
| PM / planner / final reviewer | the session itself — prefer Fable, else the newest Opus | You can't pick your own model: if the session runs on something weaker, suggest the user switch before the pipeline starts (once; respect a no). Never run the PM on a small model. |
| Creative tasks: translation, content writing, UI/design | `opus` sub-agents | |
| Other code tasks: app fixes, tooling, runners | `sonnet` sub-agents | |
| Setup coordinator (scopes, writes toolchain definitions + recipes, splits work, verifies workers) | `opus` preferred, `sonnet` acceptable — **confirmed by the user** | The job needs real reasoning (definition design, diagnosing failed installs). Propose per the rule below and let the user confirm or change it. |
| Install workers | `haiku` or `sonnet`, chosen per sub-task by the coordinator | `haiku`: mechanical — one exact, guarded command (a package-manager install, a plain recipe case) whose only failure mode is "report it". `sonnet`: a sub-task that may need diagnosis within its brief — a private env with a build step (Gradle warm-up, `cargo fetch` of real crates, pip packages with native extensions), a multi-step recipe, a source-built tool. |

**Choosing a sub-agent's model** — a child never runs on a stronger model than its parent (e.g. no `opus` coordinator under a `sonnet` PM). For the coordinator:

- PM on Fable or Opus → propose `opus`; offer `sonnet` as the cheaper option when the scope is presets only.
- PM on Sonnet → propose `sonnet`, and mention that switching the session to Opus/Fable would allow an `opus` coordinator.
- Ask once, batched with the gate that approves the installs (bootstrap: Gate 1) — only when a coordinator will actually be spawned. An explicit user choice stands for the rest of the session.

## Reporting — push, never poll

- **A child reports by finishing.** Its final message is the report, delivered to the parent automatically (inline, or as a "hand-back" agent message — either way it is the report). That delivery *is* the ping.
- **How to spawn depends on who the parent is:**
  - *The session (PM)* spawns long jobs **in the background** (`run_in_background: true`) and keeps working with the user or ends its turn; the completion notification wakes it.
  - *A sub-agent* (e.g. the setup coordinator) spawns its children **in the foreground, all of a batch in ONE message** (`run_in_background: false`). They run concurrently and every report reaches it before it continues. A sub-agent that ends its turn to wait for background children is handed back early with a partial report — its parent then gets woken twice.
- **Parents never observe**: no polling, no sleeping-and-checking, no tailing a child's output or transcript, no "status?" messages.
- **Reports are compact**: a status table (item → done / failed / needs user) plus evidence only for failures (exact command + relevant output lines). No install logs, no narration.
- **Anomalies**: a child that hits something outside its brief (a definition bug, a missing permission, a contradiction) stops that item and reports it — it doesn't improvise a fix above its level. Long-running pipelines also append one line per anomaly to `<scratchpad>/incidents.md` (`<who>: <what> — <blocked|worked around>`), which the parent reads when it wakes.
- **The user is reached only through the PM.** Background children can't ask the user; anything needing the user (password, GUI dialog, a denied permission, a judgment call) goes up in the report and the PM asks. A child never works around a denied permission.
- **Don't trust, verify**: every parent re-checks its children's claims with its own cheap objective check before reporting up (e.g. `go run . toolchains verify`, `git status`, builds), as each skill specifies.

## Every brief

Children know only their brief. Besides the task, each one gets:

- the project root as an absolute path, and the skill/reference files to read by absolute path;
- its sandbox (which files it may write) and the report format above;
- shell hygiene: the user's shell may alias common commands (`rm` → `rm -i`, `cat` → a pager, `ls` → something interactive), which hangs non-interactive runs — use `command rm -f`, `command cat`, `find` instead of `ls` in scripts.

## When not to delegate

Spawning costs a round trip. A level does small jobs itself — e.g. setup with at most one toolchain to install and no custom environment. If the environment can't spawn nested sub-agents, the level that can't spawn does its children's work itself, sequentially, with the same rules.
