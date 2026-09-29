---
name: playbook-setup-env
description: Detect and set up the toolchains a playbook's exercises need (C compiler, Python, Node.js, Kotlin CLI, JDK). Use when playground panes fail with "toolchain not installed", before generating exercises for a new language, or when the user asks to set up / check the environment.
---

# Set up the playbook environment

Make sure every toolchain this playbook actually uses works on this machine — and install NOTHING beyond that scope.

## 1. Figure out what's needed (scope first!)

In priority order:

1. **`curriculum.md`'s `toolchains:` line** (from the `playbook-plan` skill) — the authoritative scope for a playbook being built. If it exists, install exactly this list, nothing more.
2. Otherwise scan what already ships: pane `toolchain` values in `content/chapters/*/exercises.json`, plus `default_toolchain` in `content/course.json`.
3. The running server also reports availability: `curl -s localhost:<port>/api/course` → `"toolchains": {"c": true, "python": true, …}`.

Never install the full toolchain list "just in case" — JVM installs are big and slow, and unused toolchains are pure waste. If the user asks for everything, confirm once that they really want out-of-scope installs.

**Ask, don't guess**: unclear scope, a version choice that matters, or anything that changes the user's machine beyond the obvious — ask first instead of assuming.

## 2. Check what's installed

| Toolchain | Required binaries | Check |
|-----------|-------------------|-------|
| `c` | `cc` | `cc --version` |
| `cpp` | `c++` | `c++ --version` |
| `python` | `python3` | `python3 --version` |
| `node` | `node` | `node --version` |
| `kotlin` | `kotlinc` AND `kotlin` | `kotlinc -version` (slow first run is normal) |
| `java` | `javac` AND `java` | `javac --version` |

## 3. Install what's missing (macOS)

Prefer Homebrew; tell the user what you're installing and why before running:

- `c` / `cpp`: `xcode-select --install` (Command Line Tools ship both cc and c++; needs a user-confirmed GUI dialog — hand this to the user).
- `python`: `brew install python3` (macOS usually ships one already).
- `node`: `brew install node`
- `kotlin`: `brew install kotlin` (installs the Kotlin CLI: `kotlinc` + `kotlin`; pulls a JDK dependency if none).
- `java`: `brew install openjdk` — then follow brew's caveat to symlink it into the JDK path, or the `java`/`javac` on PATH won't see it.

On Linux, use the distro package manager equivalents (`build-essential`, `python3`, `nodejs`, `kotlin` via sdkman, `default-jdk`). If a needed install requires the user's password or a GUI step, stop and hand them the exact command instead of retrying.

## 4. Verify for real

For each required toolchain, run a hello-world through the SAME commands the app's runner uses, in a temp dir:

- `c`: `cc -Wall -Wextra -O0 -o prog hello.c && ./prog`
- `cpp`: `c++ -std=c++17 -Wall -Wextra -O0 -o prog hello.cpp && ./prog`
- `python`: `python3 -u hello.py`
- `node`: `node hello.js`
- `kotlin`: `kotlinc main.kt -d prog.jar && kotlin -classpath prog.jar MainKt` (entry `main.kt` ⇒ class `MainKt`)
- `java`: `javac -d . Main.java && java Main`

Clean up temp files afterwards. Note: kotlin/java compiles are slow (seconds) — warn content authors that heavy JVM exercises make the Run button feel sluggish; prefer small single-file programs.

## 5. Report

Tell the user, per toolchain: installed & verified / newly installed / needs their action (with the exact command). If the playbook server is running, remind them a restart is NOT needed — toolchain lookup happens per Run.
