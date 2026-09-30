#!/bin/sh
# Environment recipe for macOS (Homebrew) — run from the project root.
#
#   sh env/setup-macos.sh c python           # names from content/toolchains.json
#
# Mirror of env/setup-debian.sh: the playbook-setup-env skill keeps both in
# sync, adding a case for every toolchain the playbook defines itself.
# System toolchains come from Homebrew; playbook-private things (venvs,
# libraries, prefetched dependencies) go into $ENV_DIR — its bin/ is on PATH
# for every build/run command.
set -eu

# files the recipes pin (requirements.txt, Cargo.toml, ...) live next to
# this script: refer to them as "$HERE/..." — the working directory differs
# between a local run (project root) and the Dockerfile (/)
HERE=$(cd "$(dirname "$0")" && pwd)
ENV_DIR="${PLAYBOOK_ENV_DIR:-$(go run . toolchains list | sed -n 's/^ENV_DIR=//p')}"
mkdir -p "$ENV_DIR/bin"

need() { command -v "$1" >/dev/null 2>&1; }

for t in "$@"; do
  case "$t" in
    c|cpp)
      # Command Line Tools ship cc and c++; the installer is a GUI dialog
      xcode-select -p >/dev/null 2>&1 || { echo "run 'xcode-select --install' and accept the dialog, then re-run" >&2; exit 1; } ;;
    python) need python3 || brew install python3 ;;
    node)   need node || brew install node ;;
    kotlin) need kotlinc || brew install kotlin ;;
    java)
      need javac || { brew install openjdk; echo "follow brew's caveat to symlink openjdk, or java/javac stay off PATH" >&2; } ;;
    rust|rust-cargo)
      # keep an existing rustup setup; otherwise Homebrew's rust (rustc + cargo).
      # Note: the formula pulls in Homebrew's llvm (~2 GB with rust itself);
      # `brew install rustup && rustup-init -y --profile minimal` is ~0.5 GB.
      need rustc || brew install rust ;;
    # --- playbook-specific toolchains (added by playbook-setup-env) ---
    *)
      echo "setup-macos.sh: no recipe for toolchain '$t' — add one (see the playbook-setup-env skill)" >&2
      exit 1 ;;
  esac
done
