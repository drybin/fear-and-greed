# Design

## Execution

The engine supports `long` and `short` sides. A short opens with adverse sell
slippage, closes with adverse buy slippage, and realizes `(entry - exit) *
quantity`. Its equity is isolated initial cash plus marked short PnL; entry
proceeds are never counted as free cash. Stops trigger upward and targets
trigger downward. Intrabar stop-first remains unchanged. If an upward gap
would exhaust the isolated account before an executable stop, the engine closes
at the bankruptcy price including closing commission and records `liquidation`;
the account never carries negative equity.

## Causality

For hour `t`, compare its high to only hours `[t-20, t)`. The signal becomes
known when hour `t` closes and can fill at `t+1` open. Funding is applied at a
candle timestamp before newly eligible entries, avoiding a retroactive charge
on a position first opened at that timestamp.

## Workflow

`scripts/run_futures_local_high_reversal_v1.sh` fetches missing hourly futures
and funding CSV files, verifies code, prepares a futures manifest, runs
development, freezes it, then writes the development review. Holdout remains
locked and must be opened explicitly through the normal final phase.
