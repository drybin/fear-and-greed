## 1. Frozen inputs

- [x] 1.1 Add validated `regime_mode` to the portfolio manifest identity
- [x] 1.2 Expose the mode through `portfolio-prepare`
- [x] 1.3 Produce a distinct candidate label for each mode

## 2. Evaluation

- [x] 2.1 Implement `both`, `btc-ema`, `breadth`, and `none` regime semantics
- [x] 2.2 Add deterministic mode tests
- [x] 2.3 Add a sequential no-fetch workflow script

## 3. Research run

- [ ] 3.1 Commit the implementation and rebuild `bin/cli`
- [ ] 3.2 Run the four frozen manifests against the existing candle cohort
- [ ] 3.3 Compare reports before any follow-up hypothesis
