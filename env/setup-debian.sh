#!/bin/sh
# Environment recipe for Debian/Ubuntu hosts — the Dockerfile runs it as root.
#
#   sh env/setup-debian.sh c python          # names from content/toolchains.json
#
# The cases below cover the template's preset toolchains. The
# playbook-setup-env skill adds a case for every toolchain the playbook
# defines itself (build tools, libraries, venvs, prefetched dependencies)
# and mirrors it in env/setup-macos.sh, so local and hosted playbooks run the
# same definitions. Playbook-private things go into $ENV_DIR — its bin/ is on
# PATH for every build/run command.
set -eu

# files the recipes pin (requirements.txt, Cargo.toml, ...) live next to
# this script: refer to them as "$HERE/..." — the working directory differs
# between a local run (project root) and the Dockerfile (/)
HERE=$(cd "$(dirname "$0")" && pwd)
ENV_DIR="${PLAYBOOK_ENV_DIR:-/opt/playbook-env}"
mkdir -p "$ENV_DIR/bin"

need() { command -v "$1" >/dev/null 2>&1; }
# apt holds a global lock: only touch it when something is actually missing,
# so re-runs (and parallel per-toolchain runs after the system packages are
# in) stay off the package manager
APT_UPDATED=
apt_install() {
  [ -n "$APT_UPDATED" ] || { apt-get update; APT_UPDATED=1; }
  apt-get install -y --no-install-recommends "$@"
}

for t in "$@"; do
  case "$t" in
    c)      need cc || apt_install gcc libc6-dev ;;
    cpp)    need c++ || apt_install g++ libc6-dev ;;
    python) need python3 || apt_install python3 ;;
    node)   need node || apt_install nodejs ;;
    java)   need javac || apt_install default-jdk-headless ;;
    kotlin)
      if [ ! -x "$ENV_DIR/bin/kotlinc" ]; then
        apt_install default-jdk-headless unzip curl ca-certificates
        curl -fsSL -o /tmp/kotlin.zip https://github.com/JetBrains/kotlin/releases/download/v2.1.0/kotlin-compiler-2.1.0.zip
        unzip -q -o /tmp/kotlin.zip -d /opt && rm /tmp/kotlin.zip
        ln -sf /opt/kotlinc/bin/kotlinc /opt/kotlinc/bin/kotlin "$ENV_DIR/bin/"
      fi ;;
    rust|rust-cargo)
      # distro rustc is too old; rustup's proxies need RUSTUP_HOME at run
      # time too (the Dockerfile sets it). CARGO_HOME stays per-user at run
      # time so cargo can write its registry cache.
      if [ ! -x /opt/cargo/bin/rustc ]; then
        need cc || apt_install gcc libc6-dev
        apt_install curl ca-certificates
        curl -fsSL https://sh.rustup.rs | RUSTUP_HOME="${RUSTUP_HOME:-/opt/rustup}" CARGO_HOME=/opt/cargo \
          sh -s -- -y --profile minimal --default-toolchain stable --no-modify-path
      fi
      ln -sf /opt/cargo/bin/rustc /opt/cargo/bin/cargo "$ENV_DIR/bin/" ;;
    # --- playbook-specific toolchains (added by playbook-setup-env) ---
    *)
      echo "setup-debian.sh: no recipe for toolchain '$t' — add one (see the playbook-setup-env skill)" >&2
      exit 1 ;;
  esac
done

[ -z "$APT_UPDATED" ] || rm -rf /var/lib/apt/lists/*
