#!/usr/bin/env bash
# Download and retain the frozen top-50 Binance Spot cohort for protocol-v2.
#
# Default data window is [2024-07-01, 2026-08-01) UTC. fetch-data treats
# --until as an inclusive calendar day, hence UNTIL=2026-07-31 below.
# Override any setting through the environment, for example:
#   SINCE=2024-08-01 UNTIL=2026-07-31 ./scripts/fetch_research_v2_top50.sh
#   FORCE=1 GOMAXPROCS=2 ./scripts/fetch_research_v2_top50.sh

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_top50.txt}"
SINCE="${SINCE:-2024-07-01}"
UNTIL="${UNTIL:-2026-07-31}"
INTERVAL="${INTERVAL:-1m}"
MARKET="${MARKET:-spot}"
FORCE="${FORCE:-0}"
EXPECTED_SYMBOLS="${EXPECTED_SYMBOLS:-50}"
export GOMAXPROCS="${GOMAXPROCS:-2}"

STATE_DIR="$DATA_DIR/.fetch-state"
LOG_FILE="$STATE_DIR/fetch.log"
FAILED_FILE="$STATE_DIR/failed.txt"
CONFIG_FILE="$STATE_DIR/config.txt"

mkdir -p "$DATA_DIR" "$STATE_DIR"
: >"$FAILED_FILE"

log() {
  local message="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"
  printf '%s\n' "$message" | tee -a "$LOG_FILE"
}

if [[ ! -f "$SYMBOLS_FILE" ]]; then
  log "ERROR: symbols file not found: $SYMBOLS_FILE"
  exit 1
fi

if [[ "$MARKET" != "spot" ]]; then
  log "ERROR: protocol-v2 core data must use MARKET=spot"
  exit 1
fi

if [[ ! -x "$BIN" ]]; then
	log "ERROR: executable CLI not found: $BIN"
	log "Build it first or pass BIN=/absolute/path/to/fear-and-greed."
	exit 1
fi

TOTAL="$(grep -vE '^[[:space:]]*(#|$)' "$SYMBOLS_FILE" | wc -l | tr -d ' ')"
if [[ "$TOTAL" != "$EXPECTED_SYMBOLS" ]]; then
  log "ERROR: expected $EXPECTED_SYMBOLS symbols, found $TOTAL in $SYMBOLS_FILE"
  exit 1
fi

CONFIG="since=$SINCE until=$UNTIL interval=$INTERVAL market=$MARKET symbols_file=$SYMBOLS_FILE symbols=$TOTAL"
if [[ -f "$CONFIG_FILE" ]]; then
  PREVIOUS_CONFIG="$(cat "$CONFIG_FILE")"
  if [[ "$PREVIOUS_CONFIG" != "$CONFIG" && "$FORCE" != "1" ]]; then
    log "ERROR: existing data was fetched with different settings"
    log "existing: $PREVIOUS_CONFIG"
    log "requested: $CONFIG"
    log "Use the original settings or rerun with FORCE=1 to replace every CSV."
    exit 1
  fi
fi
printf '%s\n' "$CONFIG" >"$CONFIG_FILE"

INDEX=0
SUCCESS=0
SKIPPED=0
FAILED=0

log "Starting protocol-v2 data fetch"
log "window=[$SINCE, day-after-$UNTIL) interval=$INTERVAL market=$MARKET symbols=$TOTAL"
log "data_dir=$DATA_DIR symbols_file=$SYMBOLS_FILE"

while IFS= read -r raw || [[ -n "$raw" ]]; do
  symbol="$(printf '%s' "$raw" | tr -d '[:space:]')"
  [[ -z "$symbol" || "$symbol" == \#* ]] && continue

  INDEX=$((INDEX + 1))
  csv="$DATA_DIR/${symbol}.csv"
  if [[ "$FORCE" != "1" && -s "$csv" ]]; then
    SKIPPED=$((SKIPPED + 1))
    log "[$INDEX/$TOTAL] SKIP $symbol: existing non-empty CSV"
    continue
  fi

  log "[$INDEX/$TOTAL] FETCH $symbol"
  if "$BIN" fetch-data \
    --symbol "$symbol" \
    --interval "$INTERVAL" \
    --market "$MARKET" \
    --since "$SINCE" \
    --until "$UNTIL" \
    --dir "$DATA_DIR" \
    --no-progress \
    >>"$LOG_FILE" 2>&1 && [[ -s "$csv" ]]; then
    SUCCESS=$((SUCCESS + 1))
    log "[$INDEX/$TOTAL] OK $symbol"
  else
    FAILED=$((FAILED + 1))
    printf '%s\n' "$symbol" >>"$FAILED_FILE"
    log "[$INDEX/$TOTAL] FAIL $symbol"
  fi
done <"$SYMBOLS_FILE"

log "Finished: downloaded=$SUCCESS skipped=$SKIPPED failed=$FAILED total=$TOTAL"
log "Log: $LOG_FILE"

if [[ "$FAILED" -gt 0 ]]; then
  log "Failed symbols: $FAILED_FILE"
  exit 2
fi

log "Data is ready for fingerprinting and canonical manifest generation."
