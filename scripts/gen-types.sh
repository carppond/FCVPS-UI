#!/usr/bin/env bash
# gen-types.sh — Generate TS types from Go structs via tygo, then diff against api.ts.
# v1 behavior: diff only (no auto-overwrite), since api.ts is the hand-crafted contract.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATED="$REPO_ROOT/web/src/types/api.generated.ts"
CONTRACT="$REPO_ROOT/web/src/types/api.ts"
TYGO_YAML="$REPO_ROOT/tygo.yaml"

echo "[gen-types] Installing tygo..."
go install github.com/gzuidhof/tygo@latest

echo "[gen-types] Running tygo..."
cd "$REPO_ROOT"
tygo generate --config "$TYGO_YAML"

echo "[gen-types] Generated: $GENERATED"

if [ ! -f "$CONTRACT" ]; then
  echo "[gen-types] WARNING: $CONTRACT does not exist. Nothing to diff."
  exit 0
fi

# Normalise both files for comparison (strip comments, blank lines, leading whitespace)
normalize() {
  grep -v '^\s*//' "$1" | grep -v '^\s*\*' | grep -v '^\s*$' | sed 's/^\s*//'
}

TMP_GEN=$(mktemp)
TMP_CON=$(mktemp)
normalize "$GENERATED" >"$TMP_GEN"
normalize "$CONTRACT"  >"$TMP_CON"

if diff -u "$TMP_CON" "$TMP_GEN" >/dev/null 2>&1; then
  echo "[gen-types] OK — generated types match api.ts"
else
  echo ""
  echo "=========================================================="
  echo "[gen-types] WARNING: generated types differ from api.ts"
  echo "  Generated: $GENERATED"
  echo "  Contract : $CONTRACT"
  echo "----------------------------------------------------------"
  diff -u "$TMP_CON" "$TMP_GEN" || true
  echo "=========================================================="
  echo ""
  echo "[gen-types] Action required: manually sync api.ts with the generated output."
  echo "[gen-types] Exiting with 1 in --strict mode."
  if [[ "${1:-}" == "--strict" ]]; then
    rm -f "$TMP_GEN" "$TMP_CON"
    exit 1
  fi
fi

rm -f "$TMP_GEN" "$TMP_CON"
echo "[gen-types] Done."
