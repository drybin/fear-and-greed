## 1. Inputs and manifest

- [ ] 1.1 Define frozen standalone signal artifact references and checksum validation
- [ ] 1.2 Define starting capital, costs, risk, notional, slot, aggregate-risk, priority, and decision fields
- [ ] 1.3 Reject non-`research-pass` inputs from primary portfolios unless explicitly diagnostic
- [ ] 1.4 Add manifest and source-integrity tests

## 2. Shared-capital engine

- [ ] 2.1 Merge bars, exits, valuations, and entries into one deterministic event stream
- [ ] 2.2 Maintain cash, reserved cash, positions, costs, and MTM equity
- [ ] 2.3 Enforce 1% trade risk and 20% position-notional defaults
- [ ] 2.4 Enforce five-position and 5% aggregate-risk defaults
- [ ] 2.5 Enforce cash sufficiency after expected costs
- [ ] 2.6 Rank simultaneous signals deterministically
- [ ] 2.7 Persist accepted and rejected allocation decisions
- [ ] 2.8 Add fixtures for slots, aggregate risk, cash exhaustion, simultaneous signals, gaps, and correlated exits
- [ ] 2.9 Add deterministic replay and accounting reconciliation tests

## 3. Metrics and benchmarks

- [ ] 3.1 Implement portfolio return, drawdown, Calmar, exposure, turnover, and costs
- [ ] 3.2 Implement concurrency, rejected-opportunity, contribution, and concentration metrics
- [ ] 3.3 Implement aligned cash, BTC, and equal-weight benchmarks
- [ ] 3.4 Define and implement frozen portfolio decision gates
- [ ] 3.5 Add golden portfolio reports and one fixture per decision gate

## 4. Relative strength

- [ ] 4.1 Freeze hypothesis, rebalance cadence, lookbacks, top-K choices, and bounded grid
- [ ] 4.2 Calculate causal returns and rebalance-time eligibility
- [ ] 4.3 Emit target holdings and translate changes into next-bar orders
- [ ] 4.4 Handle ties and membership changes deterministically
- [ ] 4.5 Add positive, negative, tie, turnover, membership, no-lookahead, and deterministic fixtures

## 5. Portfolio research run

- [ ] 5.1 Freeze source standalone artifacts and portfolio manifest
- [ ] 5.2 Run verify and development portfolio workflows
- [ ] 5.3 Freeze shortlisted allocation and relative-strength candidates
- [ ] 5.4 Evaluate portfolio holdout without reopening standalone holdout
- [ ] 5.5 Assign `portfolio-pass`, `observe`, and `reject`
- [ ] 5.6 Publish benchmarked portfolio report and validate this change
