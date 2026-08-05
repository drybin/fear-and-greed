# Core candidate adapters

This package is the closed protocol-v2 core candidate set:

- `fib-pullback-trend-v1` (3 grid points)
- `nr7-trend-breakout-v1` (3 grid points)
- `volatility-compression-breakout-v1` (24 grid points)
- `breakout-retest-long-v2` (1 grid point)

Each grid is frozen before execution and is at or below the protocol limit of
30 candidates. The adapters contain neither fold dates nor symbol-specific
parameter selection; train-only orchestration owns global selection.

## Legacy reconciliation

The legacy simulators now expose their close-confirmed `EntrySignal` trace.
Adapters translate that trace to `execution.CloseConfirmedSignal`, preserving
the legacy initial stop, TP1, TP2, and numeric entry context as diagnostics.
The trace includes entries that remain open at the end of the input, avoiding
the former completed-trade-only look-ahead trap.

Intentional protocol-v2 differences are:

| Area | Legacy | Protocol v2 |
| --- | --- | --- |
| Entry timing | Buys at the source bar close (or BR v2's following-bar open) | Emits after source close and fills at the next available primary-bar open |
| Stop / target fills | Strategy-specific close, level, and intrabar behavior | One engine: gap-aware stops, stop-first ordering, deterministic TP1 / TP2 |
| Sizing | All available cash into one legacy position | 1% equity stop-risk, 20% notional cap, rounded quantity |
| Costs | Zero cost | Commission and adverse slippage applied independently per fill |
| Accounting | Legacy realized cash, with optional end mark | Isolated MTM account with auditable cash, position value, realized/unrealized PnL |

For fib and breakout-retest, the higher timeframe observation is usable only
after its candle closes. Breakout-retest explicitly reads the prior completed
1h bar. Therefore a change to the still-forming higher-timeframe bar cannot
change an already-emitted lower-timeframe signal.
