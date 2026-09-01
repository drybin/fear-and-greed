## Frozen windows

The workflow evaluates the exact candidate from
`relative-strength-pullback-v1` on these half-open UTC intervals:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The 200-day BTC EMA warmup remains loaded causally before each range. The
locked holdout begins at `2026-05-03`, therefore it is structurally unavailable
to this workflow.

## Reproducibility

`portfolio-prepare --start --end` embeds the requested range into the
portfolio manifest before its identity hash is calculated. `portfolio-run`
then verifies every source CSV checksum as before. The final summary merely
selects metrics already written in immutable reports; it does not recalculate
trades or make a selection decision.
