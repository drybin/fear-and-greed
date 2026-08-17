## 1. Suite isolation

- [x] 1.1 Preserve core-v2 as the default suite
- [x] 1.2 Add research-v3 manifest and runner validation
- [x] 1.3 Add `prepare --suite research-v3`

## 2. Candidates

- [x] 2.1 Implement causal volatility-compression breakout v2
- [x] 2.2 Implement causal trend mean reversion v1
- [x] 2.3 Bound each grid to four declared candidates
- [x] 2.4 Add parameter, indicator warmup, prior-range, and adapter tests
- [x] 2.5 Add daily-low-zone-v1 with causal daily-level search, full target
  exit, and calendar-aware two-day deadline
- [x] 2.6 Isolate revised daily-low-zone-v1.1 in its own one-strategy suite
- [x] 2.7 Add isolated daily-low-zone-v1.2 with third-green-candle confirmation

## 3. Protocol run

- [ ] 3.1 Commit the exact v3 implementation and build the CLI
- [ ] 3.2 Prepare a new research-v3 manifest from the frozen cohort
- [ ] 3.3 Run verify, resumable development, review, and freeze
- [ ] 3.4 Review development evidence before any v3 holdout authorization
