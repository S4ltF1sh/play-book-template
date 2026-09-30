# Playbook — content file reference

All paths are relative to `content/`. Every localized file exists in EVERY locale listed in `course.json` (shown here as `vi`/`en`). All ids, slugs, and file names are English kebab-case.

## course.json (top level)

```json
{
  "app_id": "my-playbook",
  "brand": { "vi": "My Playbook", "en": "My Playbook" },
  "brand_icon": "sprout",
  "tagline": { "vi": "…", "en": "…" },
  "subtitle": { "vi": "…", "en": "…" },
  "default_toolchain": "c",
  "port": 4360,
  "locales": ["vi", "en"],
  "default_locale": "vi",
  "chapters": [ … ]
}
```

- `app_id` names the user-data dir (progress DB). Set once per playbook; NEVER rename later.
- `default_toolchain`: a toolchain name from `toolchains.json` — used by scratch panes and panes without their own `toolchain`.
- `port`: default server port for this playbook (pick a unique one per playbook; `--port` still overrides).
- `brand_icon` (optional): line icon shown before the brand and used as the favicon — `sprout` (default) | `book` | `code` | `terminal` | `flask` | `cpu` | `globe` | `layers` | `none`. Pick one that fits the subject.

## File checklist for chapter `chNN`

```
content/course.json                     # update the chNN entry
content/chapters/chNN/
  sections/<section-id>.vi.md           # one pair per reading section
  sections/<section-id>.en.md
  quizzes/<quiz-id>.vi.json             # one pair per inline quiz
  quizzes/<quiz-id>.en.json
  exercises.json                        # all exercises for the chapter (localized inline)
  starter/<file>                        # starter code (any supported language)
  solution/<file>                       # solved versions for each exercise's solution.files
  summary.vi.md / summary.en.md
  glossary.vi.json / glossary.en.json
  quiz.vi.json / quiz.en.json           # chapter-end final quiz
  assets/<img>.svg                      # optional, served at /assets/chNN/<img>
```

## course.json chapter entry

```json
{
  "id": "chNN",
  "slug": "english-slug",
  "title": { "vi": "…", "en": "…" },
  "status": "ready",
  "sections": [
    { "id": "reading-id", "title": { "vi": "…", "en": "…" } },
    { "id": "quiz-something", "type": "quiz", "quiz": "<quiz-id>", "title": { "vi": "Quiz nhanh: …", "en": "Quick quiz: …" } },
    { "id": "exercise-id", "type": "exercise", "exercise": "<exercise-id>", "title": { "vi": "Bài tập: …", "en": "Exercise: …" } }
  ],
  "has_quiz": true, "has_exercises": true, "has_summary": true
}
```

- Section order = learning flow. Quizzes/exercises sit right after their theory. Summary/glossary/final quiz are NOT sections — they are served by `has_summary`/`has_quiz` and always come last in the nav chain (sections → summary → quiz → next chapter).
- No emoji anywhere in titles or JSON copy (template style rule): the shell draws its own line icons, and emoji clash with them.
- Section `id` becomes the progress key `chNN/<id>` — never rename once shipped.
- `"status": "planned"` shows the chapter greyed out; flip to `"ready"` when complete.

## Inline quiz file (`quizzes/<quiz-id>.<lang>.json`)

2–4 questions checking just-read theory. Gate = perfect score (retry allowed), so keep it short and fair.

```json
{
  "id": "<quiz-id>",
  "questions": [
    { "id": "m1", "prompt": "…?", "choices": ["…", "…", "…", "…"], "answer": 1, "explanation": "…" }
  ]
}
```

The `answer` index MUST be identical across locales for the same question id.

## Final quiz (`quiz.<lang>.json`)

Same question schema, `"id": "chNN"`, 8–10 questions covering the whole chapter. Balance answer lengths; vary the correct index.

## exercises.json

```json
[
  {
    "id": "<exercise-id>",
    "title":  { "vi": "…", "en": "…" },
    "brief":  { "vi": "1–3 câu bối cảnh", "en": "…" },
    "tasks":  { "vi": "- bước 1\n- bước 2\n- sửa code X rồi chạy lại\n- thử case hỏng (cuối cùng)", "en": "- …" },
    "panes": [
      { "name": { "vi": "Client", "en": "Client" }, "files": ["client.py"], "toolchain": "python", "default_args": "localhost" },
      { "name": { "vi": "Server", "en": "Server" }, "files": ["server.c"], "toolchain": "c", "default_args": "" }
    ],
    "solution": {
      "notes": { "vi": "ý chính + nhắc: nếu cách giải khác mà checks không tick thì dùng nút hoàn thành", "en": "…" },
      "files": ["server.c"]
    },
    "checks": [
      { "pane": 1, "pattern": "server: waiting", "label": { "vi": "…", "en": "…" } },
      { "pane": 0, "pattern": "client: received", "label": { "vi": "…", "en": "…" } }
    ]
  }
]
```

- `toolchain` per pane: a name from [`toolchains.json`](#toolchainsjson). Omitted ⇒ course `default_toolchain`, or inferred from the entry file (first definition whose `sources` match).
- **The FIRST file in `files` is the entry point** (the file that is run / holds `main`).
- How a pane builds and runs is entirely its toolchain's definition — read it in `toolchains.json` before writing starters (entry-file naming, which files get staged, what the build step prints). Pane file names may include subdirectories (`src/lib.rs`) but no spaces, `..` or absolute paths.
- `pane` in a check is the index into `panes`; `pattern` is a JS regex tested against that pane's accumulated stdout/stderr **since its last Run** (see check semantics in SKILL.md).
- Convention: Client pane first, Server second (UI shows content → client → server left-to-right).
- Starter files listed in `files` must exist in `starter/`. Single-pane exercises are fine.
- Checks must pass by following the tasks with the starter as given (after the edits the tasks ask for).
- `solution` is REQUIRED: `files` name solved versions in `solution/` (only the files the tasks change); `notes` explain the key insight in every locale and end with the "use solution & mark as done" fallback reminder.

## toolchains.json

A JSON array; each entry tells the playground how to stage a pane's files and build/run them. Order = extension-inference priority. The runner knows no languages beyond this file; the `playbook-setup-env` skill writes custom entries and installs what they need.

```json
{
  "name": "cpp-make",
  "label": "C++ (make)",
  "detect": ["make", "c++"],
  "sources": ["*.cpp"],
  "stage": ["*.hpp", "Makefile"],
  "layout": { "*.hpp": "include" },
  "workspace": "persistent",
  "env": { "CXXFLAGS": "-std=c++17 -Wall -Iinclude" },
  "build": "make -s prog",
  "run": "exec ./prog \"$@\"",
  "scratch": [
    { "name": "main.cpp", "content": "#include <iostream>\nint main(){std::cout<<\"hello, playground!\\n\";}\n" },
    { "name": "Makefile", "content": "prog: main.cpp\n\t$(CXX) $(CXXFLAGS) -o prog main.cpp\n" }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `name` | id used by panes, `default_toolchain`, and the env recipes (`env/setup-*.sh` cases) |
| `label` | display name (optional) |
| `detect` | executables that must exist for the toolchain to count as installed; bare names are looked up in `$ENV_DIR/bin`, then `PATH`; values with `/` are paths (`$ENV_DIR/venv/bin/python`) |
| `sources` | glob patterns (on the base name) of compiled/executed files. First source in a pane = `$ENTRY`; all = `$SRCS` (space-separated) |
| `stage` | patterns of extra files written but never treated as sources (headers, data, manifests). Files matching neither list are dropped |
| `layout` | pattern → subdirectory to stage into (e.g. `{"*.rs": "src"}` for Cargo). Default: workdir root |
| `workspace` | `temp` (default; fresh dir per Run) or `persistent` (kept per pane across Runs so incremental builds are fast; identical files aren't rewritten, so mtimes stay put) |
| `env` | extra variables; values expand `$ENV_DIR`, `$PATH`, … (e.g. `"PATH": "$ENV_DIR/venv/bin:$PATH"`) |
| `build` | optional `sh` command run first; its output streams to the pane; non-zero exit = "compile failed"; 5-minute timeout |
| `run` | `sh` command starting the program in a pty; pane args are `"$@"` — end it with `exec … "$@"` |
| `scratch` | starter files for the free playground (first = entry); `playbook toolchains verify` runs it as the smoke test |

Every command sees: `$ENTRY`, `$SRCS`, `$ENV_DIR` (the playbook's private environment; `--env-dir` / `$PLAYBOOK_ENV_DIR` / default `<data>/env`), and `PATH` with `$ENV_DIR/bin` first.

More patterns:

```json
{ "name": "python-ds", "label": "Python (data)", "detect": ["$ENV_DIR/venv/bin/python"],
  "sources": ["*.py"], "stage": ["*.csv"],
  "env": { "PATH": "$ENV_DIR/venv/bin:$PATH" },
  "run": "exec python -u \"$ENTRY\" \"$@\"" }

{ "name": "kotlin-gradle", "label": "Kotlin (Gradle)", "detect": ["gradle", "java"],
  "sources": ["*.kt"], "stage": ["build.gradle.kts", "settings.gradle.kts"],
  "layout": { "*.kt": "src/main/kotlin" }, "workspace": "persistent",
  "build": "gradle -q --offline installDist",
  "run": "exec build/install/app/bin/app \"$@\"" }
```

- `python-ds`: setup creates `$ENV_DIR/venv` and installs pinned libraries (`env/requirements.txt`).
- `kotlin-gradle`: `settings.gradle.kts` sets `rootProject.name = "app"`; setup runs one online build so `--offline` has the dependencies cached.
- `rust-cargo` (preset): Cargo layout via `layout`, `cargo build --offline`; crates used by exercises are prefetched during setup (`cargo fetch`).

## glossary file

```json
[ { "term": "english term", "def": "định nghĩa ngắn (per-locale file)" } ]
```

5–15 terms per chapter, `term` always English.

## Inline markdown in JSON copy

These JSON fields are rendered with a small inline-markdown subset (NOT full markdown — that is only for section `.md` files):
`brief`, `tasks`, `solution.notes`, check `label`, quiz `prompt` / `choices` / `explanation`, glossary `def`.

| Supported | Syntax |
|-----------|--------|
| code span | `` `x` `` |
| bold / italic | `**x**`, `*x*`, `_x_` |
| link | `[text](https://…)` (external opens in a new tab), `[text](#/…)`, `[text](/assets/…)` |
| paragraphs | blank line (`\n\n`) in `brief` / `notes`; a single `\n` is a line break |
| task list | one task per line in `tasks`; leading `- `, `* `, `1. `, `- [ ] ` markers are stripped (the UI draws its own bullet) |

- NOT supported there: headings, nested lists, tables, block quotes, fenced code, images, raw HTML (HTML is escaped and shows as text).
- ALWAYS wrap identifiers, commands, file names, and code in backticks. Unwrapped `*`/`_` are left literal only when they don't hug text on both sides (`a * b`, `f(*args, **kwargs)` are safe; `*ptr*` becomes italic).
- The UI labels quiz choices A, B, C, D (in `choices` order), so an `explanation` may refer to them by letter. Letters depend on order: keep `choices` order identical across locales, like `answer`.

## Section markdown conventions

- Start with `# Title`. GFM tables, fenced code blocks with the right language tag, blockquotes for source quotes.
- External links to official docs/man pages/RFCs/Wikipedia; they open in a new tab automatically.
- Images go in `assets/` and are referenced as `/assets/chNN/<file>`.

## Verification checklist (after build + restart)

1. `go build -o <binary> .` passes (only trust the build, not stale LSP diagnostics).
2. `curl -s localhost:<port>/api/course | python3 -m json.tool > /dev/null` — manifest parses; new chapter status "ready"; response includes `toolchains` availability map.
3. `curl -s "localhost:<port>/api/section/chNN/<id>?lang=<l>"` returns distinct HTML for every locale of every reading section.
4. `curl -s localhost:<port>/api/chapter/chNN/quiz` (and `/quiz/<quiz-id>`) return questions WITHOUT `answer` fields.
5. In the browser: open each exercise section, actually Run the panes and confirm every check ticks and Next unlocks; open the sample-solution disclosure and confirm copy-into-editor works; submit the inline quiz with a perfect score and confirm it unlocks. (In Claude's browser pane, `window.confirm` is suppressed — override `window.confirm = () => true` after each reload before using copy-into-editor.)
6. Leftover process check: `pgrep -fl 'prog|<entry names>'` should be empty after stopping runs.
7. Reload once and confirm progress persisted (sidebar dots, overall counter).
