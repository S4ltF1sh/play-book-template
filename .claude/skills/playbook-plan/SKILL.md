---
name: playbook-plan
description: Read document-sources and produce the curriculum plan (chapters, sections, sub-sections, toolchain scope, license notes) as curriculum.md for user approval. Run this BEFORE playbook-setup-env (so only needed toolchains get installed) and BEFORE playbook-generate (which requires an approved plan). Use when the user wants to plan a playbook, review a syllabus, or asks what a source would look like as a course.
---

# Plan a playbook curriculum

Produce `curriculum.md` at the project root: the single source of truth that `playbook-setup-env` (toolchain scope) and `playbook-generate` (content plan) both consume.

## 1. Source intake

1. Inspect `document-sources/`. Usable sources are text-first (markdown/plain/HTML) and split into parts that map to chapters.
2. If sources are missing, unsplit, or in an awkward format (one giant file, PDF, …):
   - ASK the user to provide split/markdown sources if they have them.
   - If they don't, FLAG it clearly, then propose your own split (file/part → planned chapter table) inside the draft plan. The split itself is part of what the user approves.
3. Identify the source license and record it. If it restricts redistribution (e.g. CC BY-NC-ND), the plan must say the built binary+content is personal-use only.

## 2. Draft `curriculum.md`

```markdown
# <Playbook name> — curriculum

status: DRAFT            # flip to APPROVED only after the user signs off
source: <where document-sources came from>
license: <license + what it implies>
locales: vi, en
toolchains: c, python    # ONLY what the exercises below actually need
port: <unique port for this playbook>

## ch01 — <title vi> / <title en>   (source: <part file>)
| # | section id | type | toolchain | notes |
|---|-----------|------|-----------|-------|
| 1 | some-reading | reading | — | sub-sections: <a>, <b> |
| 2 | quiz-some | quiz | — | 2–3 questions on … |
| 3 | some-lab | exercise | python | 1 pane; starter <file>; checks: … |
…
```

Planning rules:

- Learn-then-do flow: quizzes/exercises sit **immediately after the theory they practice**, never bundled at the end; every chapter closes with summary & glossary + final quiz (`has_*` flags, not sections).
- **Toolchain scope is a hard budget**: list a toolchain in the header ONLY if some exercise row uses it. This is what stops `playbook-setup-env` from installing the world. Prefer fewer toolchains unless the subject demands more; JVM toolchains (kotlin/java) make Run feel sluggish — flag that trade-off if you propose them.
- Note per exercise: pane count (1 or 2), intended starter source, and roughly what the checks will grade — enough for the user to veto early.
- Keep coverage ambitions explicit: which source parts are skipped/merged and why.

## 3. Ask, don't guess

Any concern while planning — how to split an awkward source, whether to skip/merge a part, which language an exercise should use, how deep to go — ASK the user instead of picking an interpretation. Put open questions in a "Questions for you" list alongside the draft plan; don't bury silent assumptions in it.

## 4. User review gate

Show the plan (chapter tables inline in chat or point at `curriculum.md`), iterate until the user approves, then set `status: APPROVED` in the file. Downstream skills refuse to run on a DRAFT plan.
