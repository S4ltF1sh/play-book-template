# play-book-template

A plug-and-play shell for building **interactive learning playbooks**: a single Go binary serving a bilingual reader, inline quizzes, and an auto-graded code playground that compiles and runs real programs on the learner's machine.

**Why it exists:** turning a good text (a guide, a book, your own notes) into a Codecademy-style course normally means building an app. This template already *is* the app — you (or a content-generating agent) supply only the `content/` directory, and everything else — layout, navigation, language switching, progress tracking, gating, auto-grading — just works.

## Table of contents

- [Features](#features)
- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Usage](#usage)
- [Configuration](#configuration)
- [Project structure](#project-structure)
- [Toolchains](#toolchains)
- [Bundled skills](#bundled-skills-claudeskills)
- [Self-hosting with Docker](#self-hosting-with-docker)
- [Porting the backend (e.g. to Rust)](#porting-the-backend-eg-to-rust)
- [Platform support & roadmap](#platform-support--roadmap)
- [Content licensing](#content-licensing)

## Features

- **Split layout**: lesson content on the left; an always-available playground on the right, showing **1 or 2 code panes** per exercise (two independent processes — enough for client/server labs).
- **Three gated section types**: reading (mark-as-done), inline quiz (perfect score), exercise (regex checks over real program output). Chapters close with a summary, glossary, and final quiz.
- **Real code execution** through a pty: unbuffered interactive stdio, an `args:` box, stdin input, process-group cleanup.
- **Multi-toolchain**: `c`, `cpp`, `python`, `node`, `kotlin` (Kotlin CLI), `java` — per pane.
- **Multilingual content** via `.vi.md` / `.en.md` file pairs (any locale set), with a runtime language switcher.
- **Progress persistence** in SQLite (pure-Go driver — no CGO), per-playbook via `app_id`.
- **Sample solutions** behind a disclosure, with copy-into-editor and a completion fallback.
- **Agent-ready**: bundled skills plan the curriculum, set up toolchains, and generate content; a demo chapter doubles as the reference sample.

## Requirements

- Go ≥ 1.26 to build.
- Per-toolchain runtimes only for the languages your exercises use (see [Toolchains](#toolchains)) — the `playbook-setup-env` skill installs exactly what your curriculum needs.
- macOS or Linux (Windows: see [roadmap](#platform-support--roadmap)).

## Quick start

```bash
# 1. Copy the template
cp -R play-book-template my-playbook && cd my-playbook

# 2. Make it yours: app_id, brand, tagline, port, locales, default_toolchain
$EDITOR content/course.json

# 3. Build & run
go build -o my-playbook . && ./my-playbook
```

The demo chapter (`content/chapters/ch01/`) shows every content type working — including a two-pane exercise mixing Python and C. Delete it once your real chapters exist.

## Usage

The intended flow is agent-driven, as one chain or as separate steps:

| Step | Skill | What it does |
|------|-------|--------------|
| all-in-one | `playbook-bootstrap` | chains the three steps below; you only approve at the gates |
| 1. plan | `playbook-plan` | reads `document-sources/`, proposes a chapter/section syllabus + **toolchain scope** as `curriculum.md` for your approval |
| 2. environment | `playbook-setup-env` | installs & verifies **only** the toolchains the plan scopes |
| 3. content | `playbook-generate` | writes chapters (manual: one per invocation; auto: parallel sub-agents with a pre-flight brief you approve) |

Manual authoring works too: follow the schemas in `.claude/skills/playbook-generate/reference.md`, then rebuild.

## Configuration

All in `content/course.json`:

| Field | Purpose |
|-------|---------|
| `app_id` | names the user-data dir (progress DB). Set once, **never rename** |
| `brand`, `tagline`, `subtitle` | per-locale app name and home-hero text |
| `default_toolchain` | used by Scratch panes and panes without their own `toolchain` |
| `port` | this playbook's default port (`--port` overrides) |
| `locales`, `default_locale` | content languages, fallback order |

Runtime flags: `--port`, `--host` (default `127.0.0.1`; `0.0.0.0` for Docker/LAN), `--data` (data dir), `--no-open`.

## Project structure

```
main.go                  # flags, embed, data dir, serve
internal/
  content/   # manifest + localized markdown/JSON loading
  runner/    # toolchain registry; compile & run in a pty
  server/    # HTTP API + playground WebSocket
  store/     # SQLite progress/settings/attempts
web/         # SPA (vanilla JS + CodeMirror + highlight.js, vendored)
content/
  course.json            # manifest (see Configuration)
  chapters/chNN/         # sections/, quizzes/, exercises.json,
                         # starter/, solution/, summary, glossary, quiz, assets/
```

## Toolchains

| Name | Compile | Run |
|------|---------|-----|
| `c` | `cc -Wall -Wextra -O0` | `./prog` |
| `cpp` | `c++ -std=c++17 -Wall -Wextra -O0` | `./prog` |
| `python` | — | `python3 -u <entry>` |
| `node` | — | `node <entry>` |
| `kotlin` | `kotlinc … -d prog.jar` | `kotlin -classpath prog.jar <MainKt>` (`.kts`: `kotlinc -script`) |
| `java` | `javac -d .` | `java <Main>` |

Each pane declares its `toolchain` in `exercises.json` (first file listed = entry point); omitted panes use `default_toolchain` or extension inference. `/api/course` reports which toolchains are installed. Adding a language = one entry in `internal/runner/runner.go` + a CodeMirror mode in `web/vendor/` + an `EXT_META` line in `web/app.js`.

## Bundled skills (`.claude/skills/`)

`playbook-bootstrap` (the chain), `playbook-plan` (curriculum + toolchain scope), `playbook-setup-env` (scoped installs, verified with the runner's own commands), `playbook-generate` (content; auto mode shows a pre-flight brief — task→model table **and the sub-agent verification criteria** — before spawning anything). Schemas live in `playbook-generate/reference.md`.

## Self-hosting with Docker

The binary builds with `CGO_ENABLED=0` (pure-Go SQLite), so packaging is one multi-stage build. A `Dockerfile` ships with the template:

```bash
docker build --build-arg TOOLCHAINS="c python" -t my-playbook .
docker run -p 4360:4360 -v playbook-data:/data my-playbook
```

`TOOLCHAINS` installs only what your curriculum scopes. The container runs as a non-root user and binds `0.0.0.0` **inside the container** only. ⚠️ The playground executes arbitrary code by design and the app has **no authentication or per-user separation** — one container = one learner (or a trusted household). Don't expose it to the public internet.

## Porting the backend (e.g. to Rust)

The backend is deliberately small (~900 lines) and the porting contract is clean — keep these three surfaces and the frontend + all content work unchanged:

1. **HTTP API**: `/api/course`, `/api/section/{ch}/{id}`, `/api/progress`, `/api/chapter/{ch}/{summary,glossary,quiz[/{id}]}`, `/api/quiz/...{attempt,attempts}`, `/api/exercises/{ch}`, `/api/settings`, `/assets/{ch}/{name}` — same JSON shapes (see `internal/server/server.go`; note: quiz endpoints strip `answer`).
2. **WebSocket** `/ws/run`: in `{op: run|stdin|kill, toolchain, files, args, data}` / out `{type: out|status|exit|error, data, code}`, one program per socket, pty semantics, process-group kill.
3. **Storage**: SQLite tables in `internal/store/store.go`; progress keys `chNN/<section-id>` must survive the migration.

Rust ingredients that map 1:1: axum (+ tokio) for HTTP/WS, rust-embed for `go:embed`, pulldown-cmark for goldmark, rusqlite for modernc/sqlite, portable-pty for creack/pty.

## Platform support & roadmap

- **macOS / Linux**: supported (pty + process-group semantics are POSIX).
- **Windows**: not native yet — the runner leans on Unix ptys and `kill(-pid)`. Works fine under **WSL2** today. Native support (ConPTY + Job Objects) is a roadmap item; the toolchain registry is the only code that needs it.
- Roadmap ideas: editor autocomplete, notes/highlights, search, flashcards, per-playbook pane layouts.

## Content licensing

The template code is yours to reuse. The **content you generate from sources is bound by those sources' licenses** — record the license in `curriculum.md`; for restrictive licenses (e.g. CC BY-NC-ND) keep the built binary+content personal-use only and never distribute it.
