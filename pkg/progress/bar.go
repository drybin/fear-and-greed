package progress

import (
	"fmt"
	"os"

	"github.com/schollz/progressbar/v3"
)

// Bar tracks download progress to stderr.
type Bar struct {
	bar *progressbar.ProgressBar
}

// New creates a progress bar for up to total items (candles). Pass total <= 0 for indeterminate.
func New(description string, total int64) *Bar {
	if total <= 0 {
		total = -1
	}
	b := progressbar.NewOptions64(
		total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("candles"),
		progressbar.OptionThrottle(100),
		progressbar.OptionOnCompletion(func() { fmt.Fprintln(os.Stderr) }),
	)
	return &Bar{bar: b}
}

func (p *Bar) Add(n int) error {
	if p == nil || p.bar == nil {
		return nil
	}
	return p.bar.Add(n)
}

func (p *Bar) Finish() error {
	if p == nil || p.bar == nil {
		return nil
	}
	return p.bar.Finish()
}
