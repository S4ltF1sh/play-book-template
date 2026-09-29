---
name: playbook-bootstrap
description: One-shot pipeline that takes a playbook from raw document-sources to a running app - chains playbook-plan (curriculum + toolchain scope), playbook-setup-env (install only scoped toolchains), and playbook-generate (content, manual or auto mode). Use when the user wants the whole playbook built end-to-end, says "just build it", or drops sources and asks for a course.
---

# Bootstrap a playbook end-to-end

Run the three playbook skills as one continuous pipeline. The user only has to show up at the approval gates; everything else flows.

## Pipeline

```
playbook-plan  ──user approves curriculum──▶  playbook-setup-env  ──▶  playbook-generate  ──▶  verified running app
```

1. **Plan** — invoke the `playbook-plan` skill: source intake (asking for / proposing a split if sources are unformatted), draft `curriculum.md`, iterate with the user until `status: APPROVED`.
   - *Gate 1 (blocking):* the curriculum approval. Never generate content or install anything from a DRAFT plan.
2. **Environment** — invoke the `playbook-setup-env` skill for EXACTLY the plan's `toolchains:` scope. Anything that needs the user's password or a GUI step: hand them the command, continue with the toolchains that already work, and come back.
3. **Generate** — invoke the `playbook-generate` skill.
   - Ask once which mode (manual / auto); if the user already said "hands-off" or similar, default to **auto**.
   - *Gate 2 (auto mode only):* the pre-flight brief (task→model table + sub-agent verification criteria) from that skill still applies — show it and get one approval, then run to completion: generate → review → merge → build → verify → task cleanup → report.

## Rules

- **Ask, don't guess.** "One continuous pipeline" means fewer ceremony stops, NOT silent assumptions: any concern or ambiguity at any stage still goes to the user as a question. Batch questions where possible so the flow stays smooth.
- Don't re-do a stage that's already done: an APPROVED `curriculum.md` skips straight to step 2; toolchains already verified skip step 3's install.
- If a stage fails (e.g. a toolchain can't be installed), report it, adjust the plan with the user (e.g. swap the exercise language), and continue — don't silently drop planned exercises.
- The pipeline ends with the standard `playbook-generate` final report, plus one line per stage: plan approved / toolchains installed / chapters generated & verified.
