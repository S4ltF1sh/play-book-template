---
name: playbook-setup-env
description: Set up the environment a playbook's exercises need — define the toolchains in content/toolchains.json (presets for C, C++, Python, Node.js, Kotlin, Java, Rust/Cargo, or custom ones for make, Gradle, a Python venv with libraries, Cargo crates, ...), install them locally, record reproducible recipes for local and hosted (Docker) setups, and verify with the playground's own runner. Use when playground panes fail with "toolchain not installed" / "unknown toolchain", before generating exercises for a new language or build tool, or when the user asks to set up / check the environment.
---

# Set up the playbook environment

The playground is only an interface: it stages the pane's files, runs a toolchain's `build` and `run` shell commands in a pty, and forwards args and stdin. **What those commands can use is whatever this machine (local) or the container (host) provides** — and setting that up is this skill's job. Three artefacts come out of it:

| Artefact | What it is |
|----------|------------|
| `content/toolchains.json` | how each toolchain stages files and builds/runs them (schema: [playbook-generate/reference.md](../playbook-generate/reference.md#toolchainsjson)) |
| the installed environment | system packages + the playbook's private `$ENV_DIR` (venvs, libraries, prefetched deps, extra binaries in `$ENV_DIR/bin`) |
| `env/setup-macos.sh`, `env/setup-debian.sh` | reproducible recipes — another machine, or `docker build`, gets the same environment |

**Your role: setup coordinator** (see [agent hierarchy](../playbook-bootstrap/agents.md)). You own the judgment work — scope, definitions, recipes — and the final verification. The installs go to cheaper workers when that pays off (step 4). Under `playbook-bootstrap` you are a sub-agent (`opus` or `sonnet`, as the user confirmed with the PM) reporting to the PM; invoked directly by the user, you are the session itself.

## 1. Scope first

In priority order:

1. **`curriculum.md`** (from `playbook-plan`): its `toolchains:` line names the toolchains, its `environment:` block says what each needs beyond a preset (build tool, libraries, versions). This is the authoritative scope — set up exactly this, nothing more.
2. Otherwise scan what ships: pane `toolchain` values in `content/chapters/*/exercises.json`, plus `default_toolchain` in `content/course.json`.
3. Current state: `go run . toolchains list` prints `ENV_DIR=…` and installed/missing per defined toolchain. If it warns that `app_id` is still the template's, stop: a unique `app_id` must be set in `content/course.json` first (playbook-plan step 0), or everything installs into the template demo's `$ENV_DIR`.

Never install the world "just in case" — JVM and build-tool installs are big and slow. If the user asks for everything, confirm once.

**Ask, don't guess**: unclear scope, a version choice that matters, a library the source uses but doesn't name, or anything that changes the user's machine beyond the obvious — ask first (as a sub-agent: stop and put the question in your report; the PM asks the user).

**Consent**: an APPROVED `curriculum.md` whose `environment:` block lists the installs (with size estimates) is the user's go-ahead. Without one, show the user what you will install, roughly how big it is, and why — and get an OK — before installing or spawning anything.

## 2. Define the toolchains

For every toolchain in scope:

- **A preset fits** (`c`, `cpp`, `python`, `node`, `kotlin`, `java`, `rust`, `rust-cargo` ship in `content/toolchains.json`): use it as-is.
- **The source needs more** (a Makefile, Gradle, a library, crates): add a new entry — don't bend a preset, give it its own name (`cpp-make`, `kotlin-gradle`, `python-ds`, …). Follow the schema and design rules in reference.md; the examples there cover the common cases.

Rules that keep definitions healthy:

- `run` ends in `exec … "$@"` — the program replaces the shell (clean signals) and receives the pane's args intact.
- Quote `"$ENV_DIR"` everywhere: on macOS it contains a space (`~/Library/Application Support/…`).
- `detect` lists the tools **and** a marker of the private part (`$ENV_DIR/venv`, `$ENV_DIR/cargo/registry`), so a toolchain whose setup never ran shows as missing instead of failing at build time.
- **Run never touches the network.** Dependencies are installed or prefetched during setup; build commands use offline flags (`--offline`, `pip --no-index`, …).
- Build tools that compile incrementally (make, cargo, gradle) get `"workspace": "persistent"`, and quiet flags (`-q`, `-s`) so their chatter doesn't pollute check regexes.
- Playbook-private installs live in `$ENV_DIR`, referenced from definitions as `$ENV_DIR/...` — never `pip install` globally or `npm -g`.
- Array order is extension-inference priority; keep `sources` patterns of competing toolchains ordered so the common one comes first.
- Give every definition a `scratch` program that exercises what's special about it (e.g. `import numpy`, a crate `use`) — it is the smoke test in step 5.
- Cargo build scripts that compile C (the `cc` crate) need `cargo:rerun-if-changed` per C file — see reference.md's rust-cargo notes; tell `playbook-generate` in your report.
- A new file extension (`.go`, `.toml`, `.lua`, …) also needs editor support in `web/app.js`: `EXT_META` (CodeMirror mode, plain text if none) and `hlClassFor` (highlight.js language, used in solution code blocks). Add the entries; that's the only non-data change a new language needs.
- Leave unused presets in place unless the user wants a lean file; never remove one referenced by exercises or `default_toolchain`.

## 3. Write the recipes (you, not workers)

`$ENV_DIR` comes from `go run . toolchains list`. For every toolchain in scope, make sure `env/setup-macos.sh` and `env/setup-debian.sh` each have a `case` that installs everything it needs (the Dockerfile runs the Debian one as root with `PLAYBOOK_ENV_DIR=/opt/playbook-env`). Workers only run these recipes, so this is where installs are decided:

- macOS: prefer Homebrew. `c`/`cpp` need `xcode-select --install` (a GUI dialog — the user must do it). `java`: brew's openjdk symlink caveat. `rust`/`rust-cargo`: keep an existing rustup setup, else `brew install rust` — it pulls in Homebrew's llvm (~2 GB total); `brew install rustup && rustup-init -y --profile minimal` (~0.5 GB) is the lean alternative, a choice for the user.
- Linux: the distro package manager (`build-essential`, `python3`, `nodejs`, `default-jdk`, rustup for Rust — distro `rustc` is often too old).
- Playbook-private pieces into `$ENV_DIR`, e.g. `python3 -m venv "$ENV_DIR/venv" && "$ENV_DIR/venv/bin/pip" install -r "$HERE/requirements.txt"`; `cargo fetch` in a temp crate with the exercises' `Cargo.toml`; one online `gradle build` so `--offline` works later. Pin the files recipes need (`env/requirements.txt`, `env/Cargo.toml`, …) next to them and refer to them as `"$HERE/…"` (the script's own directory): the Dockerfile runs the recipe from `/`, not the project root.
- Docker runs the Debian recipe as root, then hands `$ENV_DIR` to the app user (`chown` in the Dockerfile) so tools that write their cache even offline (cargo's package lock, Gradle) keep working. Set the Dockerfile's default `ARG TOOLCHAINS` to this playbook's scope.
- **Guard system installs** (`need kotlinc || brew install kotlin`, `command -v … ||` before `apt-get`) so re-running a case — or running several cases at once after the system packages are in — never touches the package manager again.

## 4. Install

**Do it yourself** when at most one toolchain needs installing and none has a private-environment part: run `sh env/setup-<os>.sh <name>` and go to step 5.

**Otherwise delegate to workers in two phases**:

1. **System packages — one worker, only if something is missing.** Check first yourself (`command -v`, `brew list --versions <pkg>`, `dpkg -s <pkg>`); skip the phase when everything is present. Package managers hold a global lock (Homebrew, apt), so they are never parallelized: compose ONE command that installs every missing system package from the recipes (e.g. `brew install kotlin rust gradle`; Homebrew parallelizes the downloads itself) and hand it to a single worker.
2. **Private environments — one worker per toolchain, in parallel.** Each runs `sh env/setup-<os>.sh <name>` (its system part is now a guarded no-op) and then `go run . toolchains verify <name>`. Skip toolchains that have no private part: phase 1 plus your own verification covers them.

**Pick each worker's model by its sub-task** (tiers in [agents.md](../playbook-bootstrap/agents.md)): `haiku` when the brief is one exact, guarded command — the phase-1 package install, a plain recipe case; `sonnet` when the sub-task may need diagnosis within its brief — a private env with a build step (Gradle warm-up, `cargo fetch` of real crates, pip packages with native extensions), a multi-step recipe. Never above your own model.

**How to spawn**: all workers of a phase in ONE message, each with its `model` and `run_in_background: false`. They run concurrently, and every worker's report reaches you (inline or as a hand-back message) before you continue — no polling, no waiting turn. Even when you are the session yourself, keep this: the phases are short and you need the results to proceed. (Background spawns don't suit a sub-agent coordinator: it would be handed back early with a partial report, and its parent woken twice.)

**Worker brief** (self-contained — the worker knows nothing else):

- the project root and the exact commands to run, in order;
- never edit repository files, never use `sudo`, never work around a denied permission;
- password prompt or GUI dialog → stop and report the exact command for the user;
- a failure → `haiku`: at most one retry if it looks transient (network), then stop and report the exact command and the relevant output lines; `sonnet`: may also diagnose (read logs, check versions) and retry a corrected invocation of the *same* install — a definition or recipe bug still goes into the report for you to fix, never into a repository edit;
- final report = one line per command: ok / failed (+ evidence) / needs user, plus the `toolchains verify` output verbatim. No logs, no narration.

## 5. Verify for real (always you)

Don't trust worker reports — re-check with your own cheap, objective commands:

```bash
go run . toolchains verify <name> [<name>...]   # every toolchain in scope
git status --short                              # workers must not have changed anything
```

`verify` runs each toolchain's `scratch` program through the playground's own runner — same staging, env, and commands as the Run button — and prints PASS/FAIL (exit code 0 or not) followed by the program's output. Everything in scope must PASS **and** print what its scratch is meant to print — read the output, exit 0 alone isn't proof. Also run one real exercise starter per custom toolchain once it exists. A FAIL caused by a definition or recipe is yours to fix (then re-verify); one caused by the machine goes into the report.

## 6. Report

To the user, or to the PM when you are a sub-agent (compact, per [agents.md](../playbook-bootstrap/agents.md)):

- one row per toolchain: preset or custom / already present · newly installed (package + version) / verified ✓ · failed (evidence) · needs user (exact command);
- the worker models used (one line), side effects worth knowing (large dependencies, upgraded packages) and the recipe/definition files you changed;
- if the playbook server is running: new or edited `toolchains.json` entries need a rebuild + restart (content is embedded); installs alone don't — detection happens per Run.
