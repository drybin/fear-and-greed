#!/usr/bin/env bash
# Batch: top-50 (static list) → fetch 2y 1m CSV → scan all algos → rm CSV.
# Designed for a small VPS (2 vCPU / 4GB RAM / 30GB disk). Resume-safe.
#
# Usage (from repo root, after building CLI):
#   ./scripts/batch_top50_vps.sh
#   GOMAXPROCS=1 BIN=./bin/cli ./scripts/batch_top50_vps.sh
#
# Keep ALGOS in sync with scan-markets --algo Usage (not "all" = rise+drop only).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data}"
REPORT_DIR="${REPORT_DIR:-$ROOT/reports}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_top50.txt}"
MARKET="${MARKET:-spot}"
INTERVAL="${INTERVAL:-1m}"
LAST_YEARS="${LAST_YEARS:-2}"
MIN_FREE_GB="${MIN_FREE_GB:-2}"
NICE_LEVEL="${NICE_LEVEL:-10}"
export GOMAXPROCS="${GOMAXPROCS:-1}"

# Soft virtual memory cap (~3 GiB) when ulimit allows (ignore failure).
if command -v ulimit >/dev/null 2>&1; then
  ulimit -v $((3 * 1024 * 1024)) 2>/dev/null || true
fi

# Sync with internal/presentation/command/scan_markets.go --algo Usage.
ALGOS="${ALGOS:-rise,rise-2d-profit,drop,drop-margin,trend,trend-long,trend-long-sma,trend-long-sma-retest,crt-long,breakout-retest-long,breakout-retest-long-v2,fib-pullback-long,fib-pullback-long-v2,fib-pullback-trend-v1,nr7-trend-breakout-v1,volatility-compression-breakout-v1,liquidity-sweep-long,liquidity-sweep-long-v2,liquidity-sweep-long-v3,liquidity-sweep-long-v4,liquidity-sweep-long-v5}"

if [[ -z "${SINCE:-}" ]]; then
  if date -u -v-"${LAST_YEARS}"y +%Y-%m-%d >/dev/null 2>&1; then
    SINCE="$(date -u -v-"${LAST_YEARS}"y +%Y-%m-%d)" # macOS
  else
    SINCE="$(date -u -d "${LAST_YEARS} years ago" +%Y-%m-%d)" # GNU
  fi
fi

STATE_DIR="${REPORT_DIR}/batch_state"
DONE_FILE="${STATE_DIR}/done.txt"
FAILED_FILE="${STATE_DIR}/failed.txt"
LOG_FILE="${STATE_DIR}/batch_run.log"
CURRENT_LOG="${STATE_DIR}/current.log"

mkdir -p "$DATA_DIR" "$REPORT_DIR" "$STATE_DIR"
touch "$DONE_FILE" "$FAILED_FILE"

run() {
  if command -v nice >/dev/null 2>&1; then
    nice -n "$NICE_LEVEL" "$@"
  else
    "$@"
  fi
}

log() {
  local msg="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
  echo "$msg" | tee -a "$LOG_FILE"
}

is_done() {
  grep -qxF "$1" "$DONE_FILE" 2>/dev/null
}

mark_done() {
  echo "$1" >>"$DONE_FILE"
}

mark_failed() {
  echo "$1	$2" >>"$FAILED_FILE"
}

csv_path() {
  local sym="$1"
  if [[ "$MARKET" == "futures" || "$MARKET" == "future" || "$MARKET" == "fapi" ]]; then
    echo "${DATA_DIR}/${sym}_futures.csv"
  else
    echo "${DATA_DIR}/${sym}.csv"
  fi
}

disk_free_gb() {
  # Portable-ish: df -Pk on Linux/macOS usually works for the target path.
  df -Pk "$REPORT_DIR" 2>/dev/null | awk 'NR==2 {printf "%.0f", $4/1024/1024}'
}

check_disk() {
  local free
  free="$(disk_free_gb || echo 0)"
  if [[ -z "$free" || "$free" -lt "$MIN_FREE_GB" ]]; then
    log "ABORT: free disk ${free:-?}GB < MIN_FREE_GB=${MIN_FREE_GB}GB under $REPORT_DIR"
    exit 2
  fi
}

cleanup_csv() {
  local path="$1"
  rm -f "$path" || true
}

if [[ ! -x "$BIN" && ! -f "$BIN" ]]; then
  log "ERROR: CLI binary not found: $BIN (build with: go build -o bin/cli ./cmd/cli/...)"
  exit 1
fi
if [[ ! -f "$SYMBOLS_FILE" ]]; then
  log "ERROR: symbols file missing: $SYMBOLS_FILE"
  exit 1
fi

log "=== batch_top50_vps start ==="
log "BIN=$BIN GOMAXPROCS=$GOMAXPROCS SINCE=$SINCE MARKET=$MARKET INTERVAL=$INTERVAL"
log "DATA_DIR=$DATA_DIR REPORT_DIR=$REPORT_DIR SYMBOLS_FILE=$SYMBOLS_FILE"
log "ALGOS=$ALGOS"

# Base assets treated as stables / dollar-pegged — never batch these even if listed.
STABLE_BASES='^(USDT|USDC|DAI|FDUSD|TUSD|USDD|BUSD|USDE|USDY|USDP|GUSD|PYUSD|FRAX|LUSD|EURC|EURS|EURT)$'

mapfile -t SYMBOLS < <(grep -vE '^\s*(#|$)' "$SYMBOLS_FILE" | sed 's/[[:space:]]//g')

TOTAL="${#SYMBOLS[@]}"
IDX=0
for SYM in "${SYMBOLS[@]}"; do
  IDX=$((IDX + 1))
  BASE="${SYM%USDT}"
  BASE="${BASE%BUSD}"
  BASE="${BASE%USDC}"
  if [[ "$BASE" =~ $STABLE_BASES ]] || [[ "$SYM" =~ ^(USDT|USDC|DAI|BUSD) ]]; then
    log "[$IDX/$TOTAL] skip stable: $SYM"
    continue
  fi
  if is_done "$SYM"; then
    log "[$IDX/$TOTAL] skip done: $SYM"
    continue
  fi

  check_disk
  CSV="$(csv_path "$SYM")"
  log "[$IDX/$TOTAL] === $SYM ==="

  # Ensure CSV is removed even on failure / interrupt for this symbol.
  trap 'cleanup_csv "'"$CSV"'"' EXIT

  : >"$CURRENT_LOG"
  if ! run "$BIN" fetch-data \
    --symbol "$SYM" \
    --interval "$INTERVAL" \
    --market "$MARKET" \
    --since "$SINCE" \
    --dir "$DATA_DIR" \
    --no-progress \
    >>"$CURRENT_LOG" 2>&1; then
    log "[$IDX/$TOTAL] FAIL fetch: $SYM (see $CURRENT_LOG)"
    cat "$CURRENT_LOG" >>"$LOG_FILE"
    mark_failed "$SYM" "fetch"
    cleanup_csv "$CSV"
    trap - EXIT
    continue
  fi

  if [[ ! -f "$CSV" ]]; then
    log "[$IDX/$TOTAL] FAIL missing CSV after fetch: $CSV"
    mark_failed "$SYM" "missing_csv"
    trap - EXIT
    continue
  fi

  # Do not pass --html (empty = skip HTML). report-html runs once at the end.
  if ! run "$BIN" scan-markets \
    --algo "$ALGOS" \
    --symbol "$SYM" \
    --dir "$DATA_DIR" \
    --report-dir "$REPORT_DIR" \
    --last-years "$LAST_YEARS" \
    >>"$CURRENT_LOG" 2>&1; then
    log "[$IDX/$TOTAL] FAIL scan: $SYM (see $CURRENT_LOG)"
    cat "$CURRENT_LOG" >>"$LOG_FILE"
    mark_failed "$SYM" "scan"
    cleanup_csv "$CSV"
    trap - EXIT
    continue
  fi

  cleanup_csv "$CSV"
  trap - EXIT
  mark_done "$SYM"
  log "[$IDX/$TOTAL] OK: $SYM"
  cat "$CURRENT_LOG" >>"$LOG_FILE"
done

log "=== regenerating HTML ==="
if ! run "$BIN" report-html --report-dir "$REPORT_DIR" --html true >>"$LOG_FILE" 2>&1; then
  log "WARN: report-html failed (results JSON still on disk)"
fi

STAMP="$(date -u +%Y%m%d)"
ARCHIVE="${REPORT_DIR}/batch_results_${STAMP}.tar.gz"
log "=== packing $ARCHIVE ==="
TAR_ARGS=()
for f in manifest.json algorithms.json report.html chart.html; do
  if [[ -e "$REPORT_DIR/$f" ]]; then
    TAR_ARGS+=("$f")
  fi
done
[[ -d "$REPORT_DIR/data" ]] && TAR_ARGS+=(data)
[[ -d "$STATE_DIR" ]] && TAR_ARGS+=(batch_state)
if ((${#TAR_ARGS[@]} > 0)); then
  tar czf "$ARCHIVE" -C "$REPORT_DIR" "${TAR_ARGS[@]}" 2>>"$LOG_FILE" || log "WARN: tar failed"
else
  log "WARN: nothing to archive under $REPORT_DIR"
fi

log "=== batch_top50_vps done ==="
log "done=$(wc -l <"$DONE_FILE" | tr -d ' ') failed=$(wc -l <"$FAILED_FILE" | tr -d ' ') archive=$ARCHIVE"
