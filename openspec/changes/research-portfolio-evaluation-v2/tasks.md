## 1. Inputs and manifest

- [x] 1.1 Define frozen standalone signal artifact references and checksum validation
- [x] 1.2 Define starting capital, costs, risk, notional, slot, aggregate-risk, priority, and decision fields
- [x] 1.3 Reject non-`research-pass` inputs from primary portfolios unless explicitly diagnostic
- [x] 1.4 Add manifest and source-integrity tests

## 2. Shared-capital engine

- [x] 2.1 Merge bars, exits, valuations, and entries into one deterministic event stream
- [x] 2.2 Maintain cash, reserved cash, positions, costs, and MTM equity
- [x] 2.3 Enforce 1% trade risk and 20% position-notional defaults
- [x] 2.4 Enforce five-position and 5% aggregate-risk defaults
- [x] 2.5 Enforce cash sufficiency after expected costs
- [x] 2.6 Rank simultaneous signals deterministically
- [x] 2.7 Persist accepted and rejected allocation decisions
- [ ] 2.8 Add fixtures for slots, aggregate risk, cash exhaustion, simultaneous signals, gaps, and correlated exits
- [x] 2.9 Add deterministic replay and accounting reconciliation tests

## 3. Metrics and benchmarks

- [x] 3.1 Implement portfolio return, drawdown, Calmar, exposure, turnover, and costs
- [x] 3.2 Implement concurrency, rejected-opportunity, contribution, and concentration metrics
- [x] 3.3 Implement aligned cash, BTC, and equal-weight benchmarks
- [x] 3.4 Define and implement frozen portfolio decision gates
- [ ] 3.5 Add golden portfolio reports and one fixture per decision gate

## 4. Relative strength

- [x] 4.1 Freeze hypothesis, rebalance cadence, lookbacks, top-K choices, and bounded grid
- [x] 4.2 Calculate causal returns and rebalance-time eligibility
- [x] 4.3 Emit target holdings and translate changes into next-bar orders
- [x] 4.4 Handle ties and membership changes deterministically
- [ ] 4.5 Add positive, negative, tie, turnover, membership, no-lookahead, and deterministic fixtures

## 5. Portfolio research run

- [ ] 5.1 Freeze source standalone artifacts and portfolio manifest
- [ ] 5.2 Run verify and development portfolio workflows
- [ ] 5.3 Freeze shortlisted allocation and relative-strength candidates
- [ ] 5.4 Evaluate portfolio holdout without reopening standalone holdout
- [ ] 5.5 Assign `portfolio-pass`, `observe`, and `reject`
- [ ] 5.6 Publish benchmarked portfolio report and validate this change
