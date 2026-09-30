---
name: playbook-generate
description: Generate interactive playbook content (bilingual reading sections, inline quizzes, auto-graded exercises, summary/glossary/final quiz) for a project built from play-book-template. Supports manual mode (this session generates chapter by chapter) and auto mode (orchestrated sub-agents generate chapters in parallel). Use when the user asks to create a playbook, generate chapters, or turn document sources into an interactive course.
---

# Generate playbook content

Turn the approved curriculum into chapters for the app in this project (built from `play-book-template`). Read [reference.md](reference.md) for file schemas and the verification checklist before writing anything.

## Prerequisite: an APPROVED plan

This skill runs off `curriculum.md` at the project root (produced by the `playbook-plan` skill):

- No `curriculum.md`, or `status: DRAFT` → run `playbook-plan` first (or tell the user to). Do NOT invent a syllabus inline.
- Check the plan's `toolchains:` scope against the machine (`go run . toolchains list`, or `/api/course` → `toolchains` map): anything undefined or missing → run `playbook-setup-env` for exactly those toolchains before generating exercises that need them.
- Read each scoped toolchain's definition in `content/toolchains.json` before writing starters: it decides the entry-file name, which files get staged (and where), and what the build step prints.
- The plan's license note governs how you handle source text (paraphrase/translate vs. verbatim).
- A fresh copy still carries the template's demo chapter (`content/chapters/ch01/` with `hello-playground`, `template-basics`, …). Generated chapters replace it: empty `content/chapters/ch01/` before writing the new ch01, so no demo file lingers in the embedded content.

## Generation modes

Ask the user (or infer from their request) which mode:

### Manual mode

This session's agent generates ONE chapter per invocation, following reference.md, then builds and verifies end-to-end. Best for small playbooks or when the user wants to review each chapter as it lands.

### Auto mode (orchestrated sub-agents)

You are planner/PM/final reviewer ONLY — sub-agents write the content.

**Model assignment**: follow the tier table and reporting rules in [playbook-bootstrap/agents.md](../playbook-bootstrap/agents.md) (single source for all playbook skills) — creative work on `opus` sub-agents, other code tasks on `sonnet`, you on the best available model.

**Pre-flight brief (mandatory before spawning anything).** Show the user, in one message, and let them override any part of it:

1. The task→model assignment table (one row per planned sub-agent).
2. **The verification criteria the sub-agents will be held to** — spell them out so the user can tighten/loosen them up front:
   - `go run . exercises verify chNN` passes: every solution passes all its checks and every starter fails at least one, run through the real runner (same staging, env and pty as the Run button);
   - `go run . content lint` reports no errors for the chapter: JSON, locale files and parity, quiz answer/choice parity, correct-never-longest, missing starter/solution files, check patterns, emoji;
   - both locales fully written (no fallback);
   - coverage target (~85–90% of the source part) and license handling per the plan.
3. **What you (the reviewer) will re-check after they finish**: starter diffs vs. originals, check-pattern semantics, cross-locale consistency, full build + API + in-browser exercise runs, and background-task cleanup.

Spawn only after the user approves the brief (or explicitly says to use defaults).

**Parallel-safety rules (non-negotiable):**

- One agent per chapter; each agent may write ONLY inside its own `content/chapters/chNN/` directory. `content/toolchains.json` and `env/` are off-limits to chapter agents — a needed toolchain change is an incident for you (then `playbook-setup-env`).
- `course.json` is merged by YOU: each agent writes its chapter entry to the scratchpad as `chNN-course-entry.json`; you merge, build, and restart.
- Pane files go in per-exercise directories (`starter/<exercise-id>/`, `solution/<exercise-id>/`) unless several exercises really share one file (reference.md → exercises.json).
- Agents must self-test before reporting with the shipped tools — `go run . exercises verify chNN` and `go run . content lint` — never a hand-rolled harness. Both read the embedded content, and `go run` rebuilds, so they always check the files on disk.
- Tell agents: the chapter files are deliverables, not reports. If a file-writing tool refuses one (sub-agents have seen `summary.*.md` refused as "a report file"), write it with a shell heredoc instead.

**Anomaly protocol (sub-agents):** every agent's prompt must include: on hitting anything abnormal (source contradicts itself, a check can't be satisfied, a toolchain is missing, a file it needs is outside its sandbox), the agent STOPS early and reports the problem in its final output instead of improvising, and also appends one line to `<scratchpad>/incidents.md` (`chNN: <what> — <blocked|worked around>`). You (the orchestrator) check `incidents.md` every time you wake up, and address incidents before merging.

**Review (you, after agents finish):**

1. Diff starters against the original source programs where "verbatim" was required; run `go run . content lint` and `go run . exercises verify` over all chapters yourself; reason through every check pattern against the single-run semantics (below).
2. Merge course entries, build, restart, run the reference.md verification checklist including real browser runs of at least one exercise per chapter.
3. **Task hygiene:** list all background tasks/agents still running. Close every finished-but-unclosed agent task. For a leftover task that still matters, decide: rerun it with an explicit timeout, or close it and do the remainder yourself. Also `pgrep` for leftover compiled playground processes.
4. Report to the user with per-chapter flows, review findings, and what was verified.

## Content standards (do not regress)

- **Bilingual pairs**: every localized file exists in ALL locales of course.json — never let one locale fall back to another. Body text in the primary locale keeps technical terms in English; the `en` files carry a full English version.
- **File names, ids, slugs, code comments: English only.**
- **Coverage**: ~85–90% of the source part per chapter; keep the source's voice; paraphrase/translate, don't copy restricted text verbatim.
- **Keywords**: expandable terms get external links (official docs, man pages, RFCs, Wikipedia). They open in a new tab automatically.
- **Quizzes**: distractors match the correct answer in length and plausibility — the correct answer must never be the longest option; vary the correct index; every question has an `explanation`.
- **Exercises**: real runnable starters; hands-on `tasks` that make the user edit and re-run; at least one "break it" task **placed last** (see check semantics); `checks` so it auto-grades; a `solution` is REQUIRED (solved files + bilingual notes ending with the "use solution & mark as done" reminder).
- **Gating** is automatic per section `type` — just use the right type.

## Check semantics (the #1 source of broken exercises)

- The UI re-evaluates every check regex against each pane's **current** output buffer, and the buffer is **CLEARED on every Run**. Therefore all checks pointing at one pane must be satisfiable in a SINGLE run of that pane.
- Accumulating evidence (e.g. "server saw two clients") belongs on the long-running server/listener pane, never on a pane the user re-runs.
- Section `done` is **sticky**: once all checks pass, breaking things later can't un-complete it — which is exactly why break-it tasks go last.
- Panes: single-pane exercises are fine; for pairs, Client pane before Server pane. `args` are split on whitespace (no quoting).
- Output is matched with terminal colours removed and `\r\n` line ends; keep patterns portable across macOS/Linux (reference.md → Check patterns).

## Hard rules

- **Ask, don't guess.** Any concern or ambiguity — unclear source material, a judgment call the plan doesn't settle, two defensible interpretations of what the user wants — goes to the user as a question BEFORE you act on it. A wrong guess costs a rewrite; a question costs a minute. This applies to sub-agents too: their prompts must tell them to surface such questions in their report (and `incidents.md`) instead of picking silently.
- Content is embedded via `go:embed`: ALWAYS rebuild (`go build -o <binary> .`) after editing content.
- Restart pattern: stop the old background server task first, then start `./<binary> --no-open` as a new background task.
- Never rename a shipped section id (progress keys in the user's SQLite DB); unavoidable renames must migrate keys via `POST /api/progress`.
- Respect the source license; if restricted, never distribute the built binary+content.
