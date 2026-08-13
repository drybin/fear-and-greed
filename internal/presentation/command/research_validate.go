package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/orchestration"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/urfave/cli/v2"
)

// NewResearchValidateCommand exposes the irreversible protocol-v2 phases
// without changing the legacy research commands.
func NewResearchValidateCommand() *cli.Command {
	return &cli.Command{
		Name:  "research-validate",
		Usage: "run protocol-v2 verify, development, freeze, and final phases",
		Subcommands: []*cli.Command{
			verifyResearchCommand(),
			prepareResearchCommand(),
			developmentResearchCommand(),
			freezeResearchCommand(),
			reviewResearchCommand(),
			finalResearchCommand(),
		},
	}
}

func prepareResearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "prepare",
		Usage: "fingerprint frozen candles and write the canonical core manifest",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "symbols", Required: true, Usage: "frozen symbol snapshot file"},
			&cli.StringFlag{Name: "candle-dir", Required: true, Usage: "directory containing SYMBOL.csv files"},
			&cli.StringFlag{Name: "manifest", Required: true, Usage: "canonical manifest output path"},
			&cli.StringFlag{Name: "cutoff", Required: true, Usage: "exclusive UTC cutoff date (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "workdir", Value: ".", Usage: "clean repository root"},
			&cli.StringFlag{Name: "suite", Value: "core-v2", Usage: "research suite: core-v2 or research-v3"},
			&cli.Uint64Flag{Name: "seed", Value: 42, Usage: "frozen random-control seed"},
		},
		Action: func(c *cli.Context) error {
			cutoff, err := time.ParseInLocation("2006-01-02", c.String("cutoff"), time.UTC)
			if err != nil {
				return fmt.Errorf("invalid cutoff: %w", err)
			}
			source, err := manifest.GitRevision(c.String("workdir"))
			if err != nil {
				return err
			}
			if source.Dirty {
				return fmt.Errorf("prepare requires a clean worktree so the manifest identifies exact source code")
			}
			m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{
				SymbolsFile: c.String("symbols"), CandleDir: c.String("candle-dir"), OutputPath: c.String("manifest"),
				Cutoff: cutoff, Source: source, Seed: c.Uint64("seed"), Suite: c.String("suite"),
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.App.Writer, "manifest prepared: %s (%s)\n", c.String("manifest"), m.ID)
			return nil
		},
	}
}

func verifyResearchCommand() *cli.Command {
	return &cli.Command{
		Name:  "verify",
		Usage: "run protocol-v2 research verification suites",
		Flags: []cli.Flag{&cli.StringFlag{Name: "workdir", Value: ".", Usage: "repository root"}},
		Action: func(c *cli.Context) error {
			args := []string{"test", "./internal/research/...", "./internal/strategy/..."}
			command := exec.CommandContext(c.Context, "go", args...)
			command.Dir = c.String("workdir")
			command.Stdout, command.Stderr = c.App.Writer, c.App.ErrWriter
			if err := command.Run(); err != nil {
				return fmt.Errorf("research verification failed: %w\nreproduce: (cd %s && go %s)", err, command.Dir, strings.Join(args, " "))
			}
			_, _ = fmt.Fprintln(c.App.Writer, "research verification passed")
			return nil
		},
	}
}

func phaseFlags(includeRunner bool) []cli.Flag {
	flags := []cli.Flag{
		&cli.StringFlag{Name: "manifest", Required: true, Usage: "canonical protocol-v2 manifest JSON"},
		&cli.StringFlag{Name: "output", Value: "research-output", Usage: "protocol-v2 output root"},
		&cli.StringFlag{Name: "workdir", Value: ".", Usage: "repository root used to verify the frozen source revision"},
		&cli.StringFlag{Name: "source-hash", Usage: "source hash; defaults to manifest source revision hash"},
		&cli.StringFlag{Name: "data-hash", Usage: "data hash; defaults to fingerprint aggregate from --candle-dir"},
		&cli.StringFlag{Name: "candle-dir", Usage: "directory of SYMBOL.csv inputs for in-process evaluation"},
	}
	if includeRunner {
		flags = append(flags, &cli.StringFlag{Name: "runner-command", Usage: "optional external evaluator; receives unit JSON on stdin (overrides in-process runner)"})
	}
	return flags
}

func developmentResearchCommand() *cli.Command {
	return &cli.Command{
		Name: "development", Usage: "run resumable development only; holdout access is prohibited",
		Flags: phaseFlags(true),
		Action: func(c *cli.Context) error {
			m, sourceHash, dataHash, store, err := loadPhase(c)
			if err != nil {
				return err
			}
			runner, err := buildRunner(c, m, store)
			if err != nil {
				return err
			}
			report, err := orchestration.Development(c.Context, orchestration.DevelopmentOptions{
				Manifest: m, OutputDir: c.String("output"), SourceHash: sourceHash, DataHash: dataHash,
				Runner: runner, CandleStore: store, RequireDataHashMatch: store != nil && c.String("data-hash") != "",
				Progress: func(p orchestration.Progress) {
					if p.Err != nil {
						_, _ = fmt.Fprintf(c.App.ErrWriter, "failed %s: %v\n", p.Unit.Key(), p.Err)
						return
					}
					state := "completed"
					if p.Reused {
						state = "reused"
					}
					_, _ = fmt.Fprintf(c.App.Writer, "%s %d complete, %d remaining: %s\n", state, p.Completed, p.Remaining, p.Unit.Key())
				},
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.App.Writer, "development complete: %d units; run freeze explicitly\n", len(report.Units))
			return nil
		},
	}
}

func freezeResearchCommand() *cli.Command {
	flags := phaseFlags(false)
	flags = append(flags, &cli.BoolFlag{
		Name:  "existing-development",
		Usage: "freeze checksum-valid existing development without requiring the current Git revision; never runs an evaluator",
	})
	return &cli.Command{
		Name: "freeze", Usage: "write the immutable candidate bundle; does not run final",
		Flags: flags,
		Action: func(c *cli.Context) error {
			m, sourceHash, dataHash, _, err := loadPhase(c)
			if err != nil {
				return err
			}
			bundle, err := orchestration.Freeze(c.String("output"), m, sourceHash, dataHash)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.App.Writer, "freeze complete for %s; final remains locked (development report %s)\n", bundle.ExperimentID, bundle.DevelopmentReportHash)
			return nil
		},
	}
}

func finalResearchCommand() *cli.Command {
	flags := phaseFlags(true)
	flags = append(flags,
		&cli.BoolFlag{Name: "authorize-holdout", Usage: "confirm the one-time holdout opening"},
		&cli.BoolFlag{Name: "orchestration-upgrade", Usage: "allow a verified orchestration-only upgrade over the frozen evaluator"},
	)
	return &cli.Command{
		Name: "final", Usage: "explicitly open the frozen holdout once",
		Flags: flags,
		Action: func(c *cli.Context) error {
			if !c.Bool("authorize-holdout") {
				return fmt.Errorf("final requires --authorize-holdout")
			}
			m, sourceHash, dataHash, store, err := loadPhase(c)
			if err != nil {
				return err
			}
			runner, err := buildRunner(c, m, store)
			if err != nil {
				return err
			}
			if store == nil {
				return fmt.Errorf("final requires --candle-dir to verify holdout eligibility and fingerprints")
			}
			report, err := orchestration.Final(c.Context, c.String("output"), m, sourceHash, dataHash, runner, store, func(p orchestration.Progress) {
				if p.Err != nil {
					_, _ = fmt.Fprintf(c.App.ErrWriter, "failed %s: %v\n", p.Unit.Key(), p.Err)
					return
				}
				state := "completed"
				if p.Reused {
					state = "reused"
				}
				_, _ = fmt.Fprintf(c.App.Writer, "%s %d complete, %d remaining: %s\n", state, p.Completed, p.Remaining, p.Unit.Key())
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(c.App.Writer, "final completed with %d decisions; holdout cannot be opened again\n", len(report.Decisions))
			for _, decision := range report.Decisions {
				_, _ = fmt.Fprintf(c.App.Writer, "%s %s: %s\n", decision.Strategy, decision.Candidate, decision.Decision.Status)
			}
			return nil
		},
	}
}

func reviewResearchCommand() *cli.Command {
	flags := phaseFlags(false)
	flags = append(flags, &cli.BoolFlag{
		Name: "existing-development", Usage: "review checksum-valid existing development without requiring the current Git revision",
	})
	return &cli.Command{
		Name: "review", Usage: "write a compact pre-holdout development review",
		Flags: flags,
		Action: func(c *cli.Context) error {
			m, sourceHash, dataHash, _, err := loadPhase(c)
			if err != nil {
				return err
			}
			review, err := orchestration.ReviewDevelopment(c.String("output"), m, sourceHash, dataHash)
			if err != nil {
				return err
			}
			for _, strategy := range review.Strategies {
				_, _ = fmt.Fprintf(c.App.Writer, "%s %s: irreversible=%v pre-holdout-flags=%v\n", strategy.Strategy, strategy.FrozenCandidate, strategy.IrreversibleFailedGates, strategy.PreHoldoutGateFlags)
			}
			_, _ = fmt.Fprintf(c.App.Writer, "development review complete: %s/protocol-v2/%s/reports/development-review.json\n", c.String("output"), m.ID)
			return nil
		},
	}
}

func loadPhase(c *cli.Context) (manifest.Manifest, protocolv2.SHA256Hex, protocolv2.SHA256Hex, orchestration.CandleStore, error) {
	raw, err := os.ReadFile(c.String("manifest"))
	if err != nil {
		return manifest.Manifest{}, "", "", nil, err
	}
	m, err := manifest.Decode(raw)
	if err != nil {
		return manifest.Manifest{}, "", "", nil, err
	}
	var store orchestration.CandleStore
	if dir := strings.TrimSpace(c.String("candle-dir")); dir != "" {
		store = orchestration.DirCandleStore{Dir: dir}
	}
	actualSource, err := manifest.GitRevision(c.String("workdir"))
	if err != nil {
		return manifest.Manifest{}, "", "", nil, err
	}
	existingDevelopment := c.Bool("existing-development")
	orchestrationUpgrade := c.Bool("orchestration-upgrade")
	if orchestrationUpgrade {
		if actualSource.Dirty {
			return manifest.Manifest{}, "", "", nil, fmt.Errorf("orchestration upgrade requires a clean worktree")
		}
		if err := manifest.VerifyOrchestrationOnlyUpgrade(c.String("workdir"), m.Source.GitRevision); err != nil {
			return manifest.Manifest{}, "", "", nil, err
		}
	}
	useFrozenRevision := existingDevelopment || orchestrationUpgrade
	if !useFrozenRevision && (actualSource.GitRevision != m.Source.GitRevision || actualSource.Dirty != m.Source.Dirty) {
		return manifest.Manifest{}, "", "", nil, fmt.Errorf("source revision differs from manifest: current %s dirty=%t, manifest %s dirty=%t", actualSource.GitRevision, actualSource.Dirty, m.Source.GitRevision, m.Source.Dirty)
	}
	if !useFrozenRevision && actualSource.Dirty {
		return manifest.Manifest{}, "", "", nil, fmt.Errorf("research phases require a clean worktree; commit the exact source and regenerate the manifest")
	}
	sourceHash, dataHash := protocolv2.SHA256Hex(c.String("source-hash")), protocolv2.SHA256Hex(c.String("data-hash"))
	verifiedRevision := actualSource.GitRevision
	if useFrozenRevision {
		verifiedRevision = m.Source.GitRevision
	}
	derivedSourceHash := hashText(verifiedRevision)
	if sourceHash != "" && sourceHash != derivedSourceHash {
		return manifest.Manifest{}, "", "", nil, fmt.Errorf("source hash does not match verified Git revision")
	}
	sourceHash = derivedSourceHash
	if store != nil {
		verifiedDataHash, verifyErr := orchestration.PreflightDevelopment(m, c.String("output"), store)
		if verifyErr != nil {
			return manifest.Manifest{}, "", "", nil, verifyErr
		}
		if dataHash != "" && dataHash != verifiedDataHash {
			return manifest.Manifest{}, "", "", nil, fmt.Errorf("data hash does not match verified candle fingerprints")
		}
		dataHash = verifiedDataHash
	} else if dataHash == "" {
		var hashes []string
		for _, symbol := range m.Universe.Symbols {
			hashes = append(hashes, string(symbol.CandleSHA256))
		}
		dataHash = hashText(strings.Join(hashes, "\n") + "\n")
	}
	return m, sourceHash, dataHash, store, nil
}

func buildRunner(c *cli.Context, m manifest.Manifest, store orchestration.CandleStore) (orchestration.Runner, error) {
	if cmd := strings.TrimSpace(c.String("runner-command")); cmd != "" {
		return commandRunner(cmd), nil
	}
	if store == nil {
		return nil, fmt.Errorf("provide --candle-dir for in-process evaluation or --runner-command for an external evaluator")
	}
	return orchestration.NewInProcessRunner(m, store)
}

type commandRunner string

func (r commandRunner) Run(ctx context.Context, unit orchestration.Unit) ([]byte, error) {
	payload, err := jsonMarshal(unit)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "sh", "-c", string(r))
	command.Stdin = strings.NewReader(string(payload))
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("runner failed: %w", err)
	}
	return output, nil
}

func jsonMarshal(value any) ([]byte, error) {
	// Keep an evaluator contract in one place and avoid exposing filesystem
	// paths or holdout data to a development runner.
	return json.Marshal(value)
}

func hashText(value string) protocolv2.SHA256Hex {
	sum := sha256.Sum256([]byte(value))
	return protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))
}
