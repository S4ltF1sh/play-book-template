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

- `app_id` names the user-data dir (progress DB **and** the default `$ENV_DIR`). Set a unique one as the very first step of a new playbook — before any `toolchains` command, or the playbook installs into the template demo's environment (`toolchains`/`content lint` warn while it is still `playbook-template-demo`). NEVER rename it later.
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
  starter/<exercise-id>/<file>          # starter code, per exercise (preferred)
  starter/<file>                        # … or chapter-wide, for files exercises really share
  solution/<exercise-id>/<file>         # solved versions for each exercise's solution.files
  solution/<file>                       # (same lookup: per-exercise first, then chapter-wide)
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

The `answer` index MUST be identical across locales for the same question id. Naming: the quiz id names the topic (`ctypes-types`), its section id adds the prefix (`quiz-ctypes-types`).

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
- **File lookup is per exercise**: a pane file `main.py` of exercise `buffers-lab` is read from `starter/buffers-lab/main.py`, falling back to `starter/main.py` (same for `solution/`). Two exercises of a chapter that both have a `main.py`, a `Cargo.toml` or a `build.rs` MUST use the per-exercise directories — a chapter-wide file is one file shared by every exercise that names it. `content lint` warns about chapter-wide files used by several exercises.
- Single-pane exercises are fine.
- Titles (`title`, pane `name`) are plain text — no markdown, no backticks.
- Checks must pass by following the tasks with the starter as given (after the edits the tasks ask for).
- `solution` is REQUIRED: `files` name solved versions in `solution/` (only the files the tasks change); `notes` explain the key insight in every locale and end with the "use solution & mark as done" fallback reminder.

### Check patterns

A check's `pattern` is a JavaScript RegExp (no flags) tested against the pane's output of its last Run:

- Terminal escape sequences are removed first (Python ≥3.13 tracebacks, cargo and gcc colour their output under the pty), so match the words: `ValueError: bad input`.
- Lines end with `\r\n` (pty). Anchor ends as `\r?$` — or better, don't anchor.
- Error texts differ between macOS and Linux (e.g. a missing C symbol: `symbol not found` vs `undefined symbol`; `ERANGE`: `Result too large` vs `Numerical result out of range`). Match the stable part, or alternate: `(symbol not found|undefined symbol)`.
- Keep patterns to the subset Go's `regexp` also understands (no lookaround or backreferences): `exercises verify` evaluates them without node then.
- `go run . exercises verify chNN` runs every solution (must pass all checks) and starter (must fail at least one) through the real runner and prints the output when something fails.

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
| `detect` | what must exist for the toolchain to count as installed: bare names are executables looked up in `$ENV_DIR/bin`, then `PATH`; values with `/` are file or directory paths (`$ENV_DIR/venv/bin/python`, `$ENV_DIR/cargo/registry`) — list the private environment's marker too, so a toolchain whose setup never ran shows as missing |
| `sources` | glob patterns (on the base name) of compiled/executed files. First source in a pane = `$ENTRY`; all = `$SRCS` (space-separated) |
| `stage` | patterns of extra files written but never treated as sources (headers, data, manifests). Files matching neither list are dropped |
| `layout` | pattern → subdirectory to stage into (e.g. `{"*.rs": "src"}` for Cargo; `"."` = workdir root). The most specific pattern wins: an exact name beats a glob (`{"*.rs": "src", "build.rs": "."}` keeps a Cargo build script at the crate root), then the longer pattern. Default: workdir root |
| `workspace` | `temp` (default; fresh dir per Run) or `persistent` (one dir per pane, kept across Runs so incremental builds are fast; identical files aren't rewritten, so mtimes stay put; files an earlier Run staged but this one doesn't are removed) |
| `env` | extra variables; values expand `$ENV_DIR`, `$PATH`, … (e.g. `"PATH": "$ENV_DIR/venv/bin:$PATH"`) |
| `build` | optional `sh` script run first (may span several lines: `\n` in the JSON string); its output streams to the pane; non-zero exit = "compile failed"; 5-minute timeout |
| `run` | `sh` script starting the program in a pty; pane args are `"$@"` — end it with `exec … "$@"` |
| `scratch` | starter files for the free playground (first = entry); `playbook toolchains verify` runs it as the smoke test |

Every command sees: `$ENTRY`, `$SRCS`, `$ENV_DIR` (the playbook's private environment; `--env-dir` / `$PLAYBOOK_ENV_DIR` / default `<data>/env`), and `PATH` with `$ENV_DIR/bin` first. **Always quote `"$ENV_DIR"`** — on macOS it lives under `~/Library/Application Support/`, with a space.

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
- `rust-cargo` (preset): Cargo layout via `layout` (`*.rs` → `src/`, `build.rs` at the root), `cargo build --offline`, and `run` starts the built binary directly (`cargo run` would print the build's warnings a second time). Crates used by exercises are prefetched during setup (`cargo fetch`, e.g. into `CARGO_HOME="$ENV_DIR/cargo"` set in `env`).
- A **build script that compiles C** (the `cc` crate) switches off Cargo's "rerun when any package file changed" scan: every exercise `build.rs` must print `cargo:rerun-if-changed=<file>` for each C/header file, or a learner's edit to the C side is silently not rebuilt in the persistent workspace.
- A toolchain that compiles helpers before running is one definition — `build` compiles every helper, `run` starts the entry. E.g. Python calling C through `ctypes` (verified through the runner):

  ```json
  { "name": "python-ctypes", "label": "Python + C (ctypes)", "detect": ["python3", "cc"],
    "sources": ["*.py"], "stage": ["*.c", "*.h"],
    "build": "for c in *.c; do\n  [ -e \"$c\" ] || continue\n  cc -shared -fPIC -Wall -o \"lib${c%.c}.so\" \"$c\" || exit 1\ndone",
    "run": "exec python3 -u \"$ENTRY\" \"$@\"" }
  ```

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

- The chapter's first reading ends with the source attribution line (source, license); later readings don't repeat it.
- A source example that doesn't run on the installed version (internal APIs, removed features) is described, not exercised — and noted in the report.
- Start with `# Title`. GFM tables, fenced code blocks with the right language tag, blockquotes for source quotes.
- External links to official docs/man pages/RFCs/Wikipedia; they open in a new tab automatically.
- Images go in `assets/` and are referenced as `/assets/chNN/<file>`.

## Verification checklist (after build + restart)

1. `go build -o <binary> .` passes (only trust the build, not stale LSP diagnostics); `go run . content lint` reports 0 errors (JSON, locale files and parity, quiz answer/choice parity, correct-never-longest, missing starter/solution files, check patterns, emoji); `go run . exercises verify chNN` passes.
2. `curl -s localhost:<port>/api/course | python3 -m json.tool > /dev/null` — manifest parses; new chapter status "ready"; response includes `toolchains` availability map.
3. `curl -s "localhost:<port>/api/section/chNN/<id>?lang=<l>"` returns distinct HTML for every locale of every reading section.
4. `curl -s localhost:<port>/api/chapter/chNN/quiz` (and `/quiz/<quiz-id>`) return questions WITHOUT `answer` fields.
5. In the browser: open each exercise section, actually Run the panes and confirm every check ticks and Next unlocks; open the sample-solution disclosure and confirm copy-into-editor works; submit the inline quiz with a perfect score and confirm it unlocks. (In Claude's browser pane, `window.confirm` is suppressed — override `window.confirm = () => true` after each reload before using copy-into-editor.)
6. Leftover process check: `pgrep -fl 'prog|<entry names>'` should be empty after stopping runs.
7. Reload once and confirm progress persisted (sidebar dots, overall counter).
