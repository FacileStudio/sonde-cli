#!/usr/bin/env bash
set -euo pipefail
TOOL="sonde"
BOOTSTRAP="https://get.facile.studio"
BOOTSTRAP_FALLBACK="https://raw.githubusercontent.com/FacileStudio/facile/main/install.sh"
command -v facile >/dev/null 2>&1 ||
  { curl -fsSL "$BOOTSTRAP" | bash || curl -fsSL "$BOOTSTRAP_FALLBACK" | bash
    export PATH="${FACILE_BIN_DIR:-$HOME/.local/bin}:$PATH"; }
exec facile install "$TOOL" "$@"
