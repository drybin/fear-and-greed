# RR Two Exit v1

Evaluate a full-position 2R exit as a new pre-specified hypothesis after RR3 was rejected. Entries, stops, time exits, universe, costs, folds, controls, and stress tests remain unchanged for Daily Low Zone v1.3, Donchian `dc40-stop20`, EMA Pullback `ema50-stop20`, NR7 `nr7-filter-1`, and Spot Flow Pullback `flow55-pullback3`.

The target is calculated from the actual next-bar fill as `entry + 2 * (entry - stop)`. Holdout remains locked.
