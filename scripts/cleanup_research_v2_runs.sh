#!/usr/bin/env bash
# Lists or removes incomplete protocol-v2 run directories.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
CUTOFF="${CUTOFF:-2026-08-01}"
RUNS_DIR="$DATA_DIR/runs"
MODE="dry-run"
INCLUDE_CURRENT=0

for arg in "$@"; do
  case "$arg" in
    --apply) MODE="apply" ;;
    --include-current) INCLUDE_CURRENT=1 ;;
    --dry-run) MODE="dry-run" ;;
    *)
      echo "Usage: $0 [--dry-run] [--apply] [--include-current]" >&2
      exit 2
      ;;
  esac
done

if [[ ! -d "$RUNS_DIR" ]]; then
  echo "No research run directory: $RUNS_DIR"
  exit 0
fi

REVISION="$(git -C "$ROOT" rev-parse --short=12 HEAD)"
CURRENT_RUN="${CUTOFF}-${REVISION}"
found=0

for run_dir in "$RUNS_DIR"/*; do
  [[ -d "$run_dir" ]] || continue
  run_name="$(basename "$run_dir")"

  frozen="$(find "$run_dir" -type f \( -path '*/freeze/bundle.json' -o -path '*/holdout/final.json' \) -print -quit)"
  if [[ -n "$frozen" ]]; then
    echo "KEEP frozen/final: $run_dir"
    continue
  fi
  if [[ "$run_name" == "$CURRENT_RUN" && "$INCLUDE_CURRENT" != "1" ]]; then
    echo "KEEP current: $run_dir"
    continue
  fi

  found=1
  size="$(du -sh "$run_dir" | awk '{print $1}')"
  if [[ "$MODE" == "apply" ]]; then
    echo "REMOVE incomplete ($size): $run_dir"
    rm -rf -- "$run_dir"
  else
    echo "WOULD REMOVE incomplete ($size): $run_dir"
  fi
done

if [[ "$found" == "0" ]]; then
  echo "No removable incomplete runs found."
elif [[ "$MODE" == "dry-run" ]]; then
  echo "Dry run only. Re-run with --apply to remove the listed directories."
fi
