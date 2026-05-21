#!/usr/bin/env bash
# check-size.sh — Enforce file and function line-count limits (coding standards §1).
#
# Limits:
#   Go  file   : ≤ 500 lines  (exempted if header contains "Code generated")
#   Go  func   : ≤ 80  lines  (rough awk scan; nested braces may cause rare false positives)
#   TS/TSX file: ≤ 300 lines
#
# Usage:
#   ./scripts/check-size.sh           # report only, always exit 0
#   ./scripts/check-size.sh --strict  # exit 1 when violations found
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STRICT=false
[[ "${1:-}" == "--strict" ]] && STRICT=true

VIOLATIONS=()

# ---------------------------------------------------------------------------
# 1. Go file line limit (≤ 500 lines)
# ---------------------------------------------------------------------------
GO_FILE_LIMIT=500

while IFS= read -r -d '' file; do
  # Skip generated files (first 5 lines checked for "Code generated")
  if head -5 "$file" | grep -q "Code generated"; then
    continue
  fi
  lines=$(wc -l <"$file")
  if (( lines > GO_FILE_LIMIT )); then
    VIOLATIONS+=("GO_FILE  ($lines > $GO_FILE_LIMIT) $file")
  fi
done < <(find "$REPO_ROOT" -name "*.go" \
  -not -path "*/vendor/*" \
  -not -path "*/.git/*" \
  -print0)

# ---------------------------------------------------------------------------
# 2. Go function line limit (≤ 80 lines)
# ---------------------------------------------------------------------------
GO_FUNC_LIMIT=80

while IFS= read -r -d '' file; do
  # Skip generated files
  if head -5 "$file" | grep -q "Code generated"; then
    continue
  fi

  # Use awk: detect "^func " lines, count lines until next "^}" at same depth
  awk -v limit="$GO_FUNC_LIMIT" -v fname="$file" '
    /^func / {
      func_name = $0
      func_start = NR
      depth = 0
      in_func = 1
    }
    in_func {
      for (i = 1; i <= length($0); i++) {
        c = substr($0, i, 1)
        if (c == "{") depth++
        else if (c == "}") depth--
      }
      if (depth <= 0 && NR > func_start) {
        func_lines = NR - func_start + 1
        if (func_lines > limit) {
          print "GO_FUNC  (" func_lines " > " limit ") " fname " — " func_name
        }
        in_func = 0
      }
    }
  ' "$file" | while IFS= read -r line; do
    VIOLATIONS+=("$line")
  done
done < <(find "$REPO_ROOT" -name "*.go" \
  -not -path "*/vendor/*" \
  -not -path "*/.git/*" \
  -print0)

# ---------------------------------------------------------------------------
# 3. TS/TSX file line limit (≤ 300 lines)
# ---------------------------------------------------------------------------
TS_FILE_LIMIT=300

while IFS= read -r -d '' file; do
  # Skip generated files
  if head -3 "$file" | grep -q "Code generated"; then
    continue
  fi
  lines=$(wc -l <"$file")
  if (( lines > TS_FILE_LIMIT )); then
    VIOLATIONS+=("TS_FILE  ($lines > $TS_FILE_LIMIT) $file")
  fi
done < <(find "$REPO_ROOT/web/src" -name "*.ts" -o -name "*.tsx" \
  -not -path "*/.git/*" \
  -not -path "*/node_modules/*" \
  -print0 2>/dev/null)

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
if (( ${#VIOLATIONS[@]} == 0 )); then
  echo "[check-size] OK — no size violations found."
  exit 0
fi

echo ""
echo "=========================================================="
echo "[check-size] SIZE VIOLATIONS FOUND: ${#VIOLATIONS[@]}"
echo "----------------------------------------------------------"
for v in "${VIOLATIONS[@]}"; do
  echo "  VIOLATION: $v"
done
echo "=========================================================="
echo ""

if $STRICT; then
  echo "[check-size] Exiting with 1 (--strict mode)."
  exit 1
fi

echo "[check-size] Violations listed above. Run with --strict to fail CI."
exit 0
