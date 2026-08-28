# Protocol-v2 operator guide

Protocol-v2 is isolated under `research-output/protocol-v2/<experiment-id>` and
does not replace `scan-markets` or `fetch-data`.

## Fixture dry path

The small files under `testdata/research-validation-v2/` are checked-in format
fixtures, not representative market history. The orchestration test exercises
the complete development → freeze boundary using synthetic units:

```sh
go test ./internal/research/orchestration -run TestDevelopmentNeverPassesHoldoutAndFinalOpensOnce
```

## First real run

1. Pick and record a UTC cutoff. Export dated cohort files from
   `scripts/symbols_*.txt`; preserve their raw bytes and hashes.
2. Download only spot candles at the declared timeframes through that cutoff.
   Inventory every input, freeze fingerprints, eligibility output, fold ranges,
   four strategy versions/grids, base and stress costs, sizing, and gates in a
   canonical manifest.
3. Verify the source and tests:

   ```sh
   ./bin/fear-and-greed research-validate verify
   ```

   Commit the exact implementation before creating the canonical manifest.
   Every phase verifies the manifest revision against `HEAD` and rejects a
   dirty worktree, so a source hash cannot be supplied to bypass source drift.

4. Run development with the in-process evaluator (default) or an external
   `--runner-command`. Development never gives the evaluator a holdout range:

   ```sh
   ./bin/fear-and-greed research-validate development \
     --manifest manifests/core.json \
     --candle-dir ./data \
     --output research-output
   ```

   Completed units are atomically stored beside an artifact and are reused only
   when the manifest, source, data, and artifact hashes match. A changed hash
   is a stale checkpoint error, not a silent rerun.

   Preflight inventories every candle file, checks SHA-256 fingerprints against
   the manifest, and writes `eligibility.json` under the experiment report dir.
5. Review the retained development report, then explicitly freeze it:

   ```sh
   ./bin/fear-and-greed research-validate freeze \
     --manifest manifests/core.json --output research-output
   ```

   Freeze ends the convenience workflow. It never invokes final.
6. Generate and inspect the compact pre-holdout review:

   ```sh
   ./bin/fear-and-greed research-validate review \
     --existing-development --manifest manifests/core.json \
     --candle-dir ./data --output research-output
   ```

7. After explicit authorization, open the holdout once:

   ```sh
   ./bin/fear-and-greed research-validate final \
     --authorize-holdout --manifest manifests/core.json --output research-output \
     --candle-dir ./data
   ```

`final` writes its opening record before the evaluator receives the holdout
range. If interrupted, the identical invocation resumes checksum-valid holdout
unit checkpoints under that same opening; changed frozen inputs are rejected. An
external `--runner-command` may still be supplied, but `--candle-dir` remains
mandatory so the orchestrator can independently verify fingerprints and the
eligible holdout cohort. Final writes compressed base/stress holdout artifacts,
frozen gate inputs, and one decision per strategy.
