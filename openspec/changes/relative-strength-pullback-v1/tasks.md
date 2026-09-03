## 1. Entry contract

- [x] 1.1 Freeze entry mode and pullback parameters in portfolio manifest identity
- [x] 1.2 Implement causal EMA20/ATR20 entry eligibility
- [x] 1.3 Reject next-day entry gaps above the frozen maximum price
- [x] 1.4 Emit separate strategy and candidate identifiers

## 2. Workflow and verification

- [x] 2.1 Add a dedicated breadth-pullback workflow without downloading data
- [x] 2.2 Add entry eligibility and gap rejection tests
- [ ] 2.3 Commit the implementation and build the CLI
- [ ] 2.4 Run and review the frozen diagnostic
