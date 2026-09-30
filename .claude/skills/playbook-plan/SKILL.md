---
name: playbook-plan
description: Read document-sources and produce the curriculum plan (chapters, sections, sub-sections, toolchain + environment scope, license notes) as curriculum.md for user approval. Run this BEFORE playbook-setup-env (so only needed toolchains get installed) and BEFORE playbook-generate (which requires an approved plan). Use when the user wants to plan a playbook, review a syllabus, or asks what a source would look like as a course.
---

# Plan a playbook curriculum

Produce `curriculum.md` at the project root: the single source of truth that `playbook-setup-env` (toolchain + environment scope) and `playbook-generate` (content plan) both consume.

## 0. New playbook? Give it its own identity first

A fresh copy of the template still has `"app_id": "playbook-template-demo"` in `content/course.json`. `app_id` names the data dir — progress DB **and** the default `$ENV_DIR` — so until it changes, every `toolchains` command and install lands in the template demo's environment. Before anything else, set a unique kebab-case `app_id` (from the playbook's subject) and a `port` no other local playbook uses (the template demo uses 4360). Tell the user the values you picked; they go into the plan header too.

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
app_id: <unique id>      # already set in content/course.json (step 0)
port: <unique port>      # idem
toolchains: c, python-ds # ONLY what the exercises below actually need
default_toolchain: python-ds  # scratch panes + panes without their own toolchain
environment:             # what each toolchain installs — approving the plan approves these installs
  - c: present (Command Line Tools)
  - python-ds: python3 + venv with numpy, pandas (source ch03–ch05 import them) — ~150 MB, ~1 min

## ch01 — <title vi> / <title en>   (source: <part file>)
| # | section id | type | toolchain | notes |
|---|-----------|------|-----------|-------|
| 1 | some-reading | reading | — | sub-sections: <a>, <b> |
| 2 | quiz-some | quiz | — | 2–3 questions on … |
| 3 | some-lab | exercise | python | 1 pane; starter some-lab/<file>; checks: … |
…
```

Planning rules:

- Learn-then-do flow: quizzes/exercises sit **immediately after the theory they practice**, never bundled at the end; every chapter closes with summary & glossary + final quiz (`has_*` flags, not sections).
- **Toolchain scope is a hard budget**: list a toolchain in the header ONLY if some exercise row uses it. This is what stops `playbook-setup-env` from installing the world. Prefer fewer toolchains unless the subject demands more; JVM toolchains (kotlin/java) and build tools (gradle, first cargo build) make Run feel sluggish — flag that trade-off if you propose them.
- **Presets vs. custom toolchains**: `content/toolchains.json` ships presets (`c`, `cpp`, `python`, `node`, `kotlin`, `java`, `rust`, `rust-cargo`). When the source relies on more — a Makefile, Gradle, third-party libraries, crates — plan a custom toolchain with its own name (`cpp-make`, `kotlin-gradle`, `python-ds`) and describe its needs in `environment:` from what the source actually uses (tools, libraries, versions). Don't strip what the source teaches (e.g. Cargo in a Rust course) just to stay on a preset; don't add a build tool the source doesn't use.
- **The `environment:` block is the install consent.** Check the machine first: `go run . toolchains list` covers the defined toolchains; for planned custom ones, check their underlying tools yourself (`command -v`, `--version`, e.g. `go version`, `cargo --version`, `cc --version`, a library via `brew list --versions` / `dpkg -s`). List, per scoped toolchain, either "present" or what will be installed with a rough size/time estimate. Surface real choices as questions (e.g. Rust via `brew install rust`, ~2 GB with llvm, vs. rustup, ~0.5 GB). `playbook-setup-env` installs exactly this list without asking again.
- Note per exercise: pane count (1 or 2), intended starter source, and roughly what the checks will grade — enough for the user to veto early. Starter files live per exercise (`starter/<exercise-id>/main.py`), so two labs of a chapter can both have a `main.py`.
- Keep coverage ambitions explicit: which source parts are skipped/merged and why. Coverage (~85–90%) is counted on the teaching parts of a source; mark reference/API-listing parts (e.g. the second half of a library manual) as **"weave in where used"** — they are drawn on by the chapters that need them, not rewritten entry by entry.
- Source examples depend on versions: if one relies on internals or features the installed version lacks, plan it as "describe, don't run".

## 3. Ask, don't guess

Any concern while planning — how to split an awkward source, whether to skip/merge a part, which language an exercise should use, how deep to go — ASK the user instead of picking an interpretation. Put open questions in a "Questions for you" list alongside the draft plan; don't bury silent assumptions in it.

## 4. User review gate

Show the plan (chapter tables inline in chat or point at `curriculum.md`), iterate until the user approves, then set `status: APPROVED` in the file. Downstream skills refuse to run on a DRAFT plan.
