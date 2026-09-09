## Decision

**Rejected before holdout.** The five-window development experiment completed
on 2026-09-09. All twenty candidate/window reports returned `reject`; no
locked-holdout command was run or authorized.

The immutable summary is stored on the research host at:

`/home/drybin/fear-and-greed/data/research-v2/portfolio-runs/defensive-slow-momentum-walk-forward-c6d270e89367/summary.json`

## Results

Net return by fixed development window, after base execution costs:

| Candidate | 2025-02-01 to 2025-05-03 | 2025-05-03 to 2025-08-06 | 2025-08-06 to 2025-11-04 | 2025-11-04 to 2026-02-03 | 2026-02-03 to 2026-05-03 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 60d/top-5 | 0.00% | -41.19% | -10.37% | 0.00% | 0.00% |
| 60d/top-10 | 0.00% | -36.18% | -6.13% | 0.00% | 0.00% |
| 90d/top-5 | -7.26% | -6.02% | -28.10% | 0.00% | 0.00% |
| 90d/top-10 | -7.22% | -3.69% | -17.96% | 0.00% | 0.00% |

The best aggregate candidate was `90d/top-10`, but it was negative in all
three windows where it traded. Its `2025-08-06` to `2025-11-04` return was
`-17.96%`, versus BTC `-6.89%` and equal weight `-5.27%`.

## Interpretation

- The BTC EMA-200 and 50% breadth guard did keep the portfolio in cash during
  the two final development windows, avoiding participation in the
  `2025-11-04` to `2026-02-03` BTC decline of `-26.35%`.
- Cash protection did not create a viable return source. Zero-return windows
  also fail the protocol's strict positive-stress gate.
- In the May--August window, all candidates that were materially invested
  lost money. The 60d/top-10 variant had 54.45% average exposure, 86 trades,
  -36.18% net return, and 39.84% drawdown while BTC returned +17.44%.
- Changing EMA length, breadth threshold, cadence, or other guard parameters
  after observing these results would be a new optimization exercise. It is
  explicitly out of scope for this rejected hypothesis.

## Follow-up Constraint

Do not run the locked holdout for `defensive-slow-momentum-v1`. Any future
portfolio hypothesis must use a new strategy identity, a new immutable
manifest series, and a separate OpenSpec change.
