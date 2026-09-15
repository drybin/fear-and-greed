# RR Three Exit v1

## Purpose

Test whether a fixed full-position 3R exit improves five previously researched entry rules without changing their entries, stops, filters, time exits, costs, folds, controls, or universe.

## Frozen candidates

- Daily Low Zone v1.3: `daily-low-zone-third-green-tp1pct`
- Donchian Breakout v1: `dc40-stop20`
- EMA Pullback v1: `ema50-stop20`
- NR7 Trend Breakout v1: `nr7-filter-1`
- Spot Flow Pullback v1: `flow55-pullback3`

The target is calculated after the actual next-bar fill as `entry + 3 * (entry - stop)`. The full position exits there; no partial 1R exit is retained. Development, freeze, controls, stress, and review run normally; holdout remains locked.
