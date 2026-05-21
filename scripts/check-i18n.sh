#!/usr/bin/env bash
# check-i18n.sh — Validate i18n completeness and absence of hardcoded CJK.
#
# Checks:
#   1. All 4 locales (zh-CN / en / ja / ko) have identical key sets per namespace.
#   2. No hardcoded CJK characters in web/src/ (excluding locales/ + comments).
#
# Usage:
#   ./scripts/check-i18n.sh           # report only, exit 0
#   ./scripts/check-i18n.sh --strict  # exit 1 on any failure
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCALES_DIR="$REPO_ROOT/web/src/locales"
SRC_DIR="$REPO_ROOT/web/src"
STRICT=false
[[ "${1:-}" == "--strict" ]] && STRICT=true

FAILED=false

# ---------------------------------------------------------------------------
# Helper: flatten all keys from a JSON file (jq leaf-path notation)
# ---------------------------------------------------------------------------
flatten_keys() {
  local file="$1"
  jq -r 'paths(scalars) | join(".")' "$file" 2>/dev/null | sort
}

# ---------------------------------------------------------------------------
# 1. Key-set comparison across locales
# ---------------------------------------------------------------------------
LOCALES=("zh-CN" "en" "ja" "ko")
REFERENCE="${LOCALES[0]}"  # zh-CN is the reference

echo "[check-i18n] Checking key parity across locales..."

for ns_file in "$LOCALES_DIR/$REFERENCE"/*.json; do
  ns=$(basename "$ns_file")

  ref_keys=$(flatten_keys "$ns_file")

  for locale in "${LOCALES[@]:1}"; do
    target="$LOCALES_DIR/$locale/$ns"
    if [ ! -f "$target" ]; then
      echo "  MISSING FILE: $target"
      FAILED=true
      continue
    fi

    target_keys=$(flatten_keys "$target")

    # Keys in reference but missing from target
    missing=$(comm -23 <(echo "$ref_keys") <(echo "$target_keys"))
    # Keys in target but not in reference (extra keys)
    extra=$(comm -13 <(echo "$ref_keys") <(echo "$target_keys"))

    if [ -n "$missing" ]; then
      echo "  MISSING KEYS [$locale/$ns] (present in $REFERENCE, absent in $locale):"
      while IFS= read -r key; do
        echo "    - $key"
      done <<< "$missing"
      FAILED=true
    fi

    if [ -n "$extra" ]; then
      echo "  EXTRA KEYS [$locale/$ns] (present in $locale, absent in $REFERENCE):"
      while IFS= read -r key; do
        echo "    + $key"
      done <<< "$extra"
      FAILED=true
    fi
  done
done

if ! $FAILED; then
  echo "[check-i18n] OK — all locale key sets match."
fi

# ---------------------------------------------------------------------------
# 2. Hardcoded CJK detection in source files
# ---------------------------------------------------------------------------
echo "[check-i18n] Checking for hardcoded CJK characters in web/src/..."

# Use perl for portable Unicode matching (works on macOS + Linux).
# Exclude:
#   - locales/ directory (translation tables)
#   - node_modules
#   - native-name whitelist files: language switcher / profile locale select
#     display their language names in the language's own script (UX convention,
#     see docs/_dev-cheatsheet.md §i18n native-name 白名单).
# Strip lines that are purely // comments before checking.
CJK_HITS=$(
  find "$SRC_DIR" \
    \( -name "*.ts" -o -name "*.tsx" \) \
    -not -path "*/locales/*" \
    -not -path "*/node_modules/*" \
    -not -path "*/components/layout/lang-switch.tsx" \
    -not -path "*/components/auth/profile-basic-form.tsx" \
    -print0 \
  | xargs -0 perl -ne '
      # Skip pure single-line // comment lines
      next if /^\s*\/\//;
      # Remove trailing // comment before CJK check
      s|//.*$||;
      if (/[\x{4e00}-\x{9fff}]/u) {
        print ARGV . ":" . $. . ": " . $_;
      }
    ' 2>/dev/null \
  || true
)

if [ -n "$CJK_HITS" ]; then
  echo ""
  echo "=========================================================="
  echo "[check-i18n] HARDCODED CJK FOUND:"
  echo "----------------------------------------------------------"
  echo "$CJK_HITS"
  echo "=========================================================="
  FAILED=true
else
  echo "[check-i18n] OK — no hardcoded CJK in source files."
fi

# ---------------------------------------------------------------------------
# Exit
# ---------------------------------------------------------------------------
if $FAILED; then
  if $STRICT; then
    echo "[check-i18n] Exiting with 1 (--strict mode)."
    exit 1
  fi
  echo "[check-i18n] Violations listed above. Run with --strict to fail CI."
  exit 0
fi

echo "[check-i18n] All checks passed."
exit 0
