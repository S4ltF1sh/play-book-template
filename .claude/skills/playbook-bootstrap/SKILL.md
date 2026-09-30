---
name: playbook-bootstrap
description: One-shot pipeline that takes a playbook from raw document-sources to a running app - chains playbook-plan (curriculum + toolchain/environment scope), playbook-setup-env (delegated to a user-confirmed setup coordinator that installs via cheaper workers and verifies), and playbook-generate (content, manual or auto mode), with the session acting as PM. Use when the user wants the whole playbook built end-to-end, says "just build it", or drops sources and asks for a course.
---

# Bootstrap a playbook end-to-end

Run the three playbook skills as one continuous pipeline. **You are the PM**: you talk to the user, hold the gates, delegate, and check results — you don't watch the work. The PM should run on Fable or Opus: if this session is on a weaker model, suggest the user switch before starting (once). Roles, models and the reporting protocol are in [agents.md](agents.md).

## Pipeline

```
playbook-plan ──Gate 1: plan + installs──▶ ┬─▶ setup coordinator (background, reports once) ─┐
                                           └─▶ generate prep: mode, Gate 2 brief, ready chapters ┴─▶ remaining chapters ─▶ verified app
```

1. **Plan** — invoke the `playbook-plan` skill: source intake (asking for / proposing a split if sources are unformatted), draft `curriculum.md`, iterate with the user until `status: APPROVED`.
   - Before presenting Gate 1, run one cheap, read-only check: `go run . toolchains verify <scope>` — which scoped toolchains already PASS, which are missing or not yet defined.
   - *Gate 1 (blocking):* the curriculum approval — including its `environment:` block, which is the user's consent for the installs. If anything is missing, batch into the same gate the **coordinator model** question: propose per the rule in [agents.md](agents.md) (`opus` under a Fable/Opus PM, `sonnet` under a Sonnet PM) and let the user confirm or change it. Never generate content or install anything from a DRAFT plan.
2. **Environment, delegated** — nothing missing → skip to step 3. Otherwise spawn ONE sub-agent **in the background**, on the model confirmed at Gate 1, as the setup coordinator: it invokes `playbook-setup-env` for exactly the missing part of the plan's `toolchains:` + `environment:` scope (it spawns its own `haiku`/`sonnet` install workers per sub-task and verifies them) and ends with a single compact report. Do not poll it, message it, or read its output — its completion notification wakes you.
3. **Generate, overlapping with setup** — invoke the `playbook-generate` skill right away:
   - Ask once which mode (manual / auto); if the user already said "hands-off" or similar, default to **auto**.
   - *Gate 2 (auto mode only):* show the pre-flight brief (task→model table + sub-agent verification criteria) — the user reviews it while installs run.
   - Start the chapters whose toolchains all PASSed in the pre-Gate-1 check. Chapters that need a pending toolchain wait for the coordinator's report.
4. **When the setup report arrives** — re-check cheaply yourself (`go run . toolchains verify <scope>`), then start the waiting chapters. Anything the report says needs the user (password, GUI step, a choice): ask them, and keep the unaffected chapters going. A toolchain that can't be installed: adjust the plan with the user (e.g. swap the exercise language) — don't silently drop planned exercises.
5. **Finish** per `playbook-generate`: review → merge → build → verify → task cleanup → report.

## Rules

- **Ask, don't guess.** "One continuous pipeline" means fewer ceremony stops, NOT silent assumptions: any concern or ambiguity at any stage still goes to the user as a question. Batch questions where possible so the flow stays smooth. Sub-agents can't reach the user — their questions arrive in their reports and you ask.
- **Delegate, don't supervise.** Between spawning a child and its report, spend no tokens on it. Its report plus your own cheap objective check (verify, build, `git status`) is all you need.
- Don't re-do a stage that's already done: an APPROVED `curriculum.md` skips straight to step 2; toolchains that already PASS need no coordinator at all.
- The pipeline ends with the standard `playbook-generate` final report, plus one line per stage: plan approved / toolchains installed & verified / chapters generated & verified.
