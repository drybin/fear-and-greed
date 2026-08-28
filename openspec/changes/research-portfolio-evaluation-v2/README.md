# research-portfolio-evaluation-v2

Evaluate immutable research-passed signals under shared capital and run the first portfolio-native relative-strength diagnostic.

## Implemented diagnostic

`relative-strength-long-v1@v1.0.0`, candidate `rs-90d-vol30-top5`, ranks the frozen spot universe once per week from completed daily data. It uses a 90-day return divided by 30-day realized volatility, BTC EMA200 plus breadth regime filters, top-5 allocation, rank-10 retention, and two-ATR stops.

The run uses one account, 1% risk per trade, 20% maximum notional per position, five slots, 5% aggregate initial risk, and the source protocol-v2 base/stress costs. It reports cash, BTC, and equal-weight benchmarks together with an auditable allocation trail.

## Workflow

After committing and rebuilding `bin/cli`, create the immutable diagnostic manifest from the protocol-v2 manifest used for the current universe:

```bash
./bin/cli research-validate portfolio-prepare \
  --research-manifest /path/to/protocol-v2/manifest.json \
  --manifest /path/to/portfolio/manifest.json \
  --workdir /home/drybin/fear-and-greed
```

Then run the bounded experiment against the existing CSV files; it does not download data:

```bash
GOMEMLIMIT=512MiB GOGC=20 ./bin/cli research-validate portfolio-run \
  --manifest /path/to/portfolio/manifest.json \
  --candle-dir /home/drybin/fear-and-greed/data/research-v2 \
  --output /path/to/portfolio/report.json \
  --workdir /home/drybin/fear-and-greed
```

The output is immutable. Repeating the exact run is allowed; changing the source, candles, parameters, or result requires a new manifest/output path.
