package cmd

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

const (
	progressAuto   = "auto"
	progressAlways = "always"
	progressNever  = "never"
)

func validateProgress(p string) error {
	switch p {
	case progressAuto, progressAlways, progressNever:
		return nil
	default:
		return fmt.Errorf("invalid --progress %q: must be auto, always, or never", p)
	}
}

// resolveProgressEnabled returns whether progress output should be active.
// auto: only for table format on an interactive stderr.
// always: yes, unless debug is set (which makes interleaved output unreadable).
// never: no.
func resolveProgressEnabled(mode, format string, debug bool) bool {
	if debug {
		return false
	}
	switch mode {
	case progressNever:
		return false
	case progressAlways:
		return true
	default: // auto
		return format == formatTable && term.IsTerminal(int(os.Stderr.Fd()))
	}
}

// progressReporter writes inline status to stderr, overwriting the same line
// on an interactive terminal. On a non-terminal it writes newline-terminated
// lines (safe for log redirection). All methods are no-ops when disabled.
type progressReporter struct {
	w       io.Writer
	enabled bool
	isterm  bool
	lastLen int
}

func newProgressReporter(mode, format string, debug bool) *progressReporter {
	return &progressReporter{
		w:       os.Stderr,
		enabled: resolveProgressEnabled(mode, format, debug),
		isterm:  term.IsTerminal(int(os.Stderr.Fd())),
	}
}

func (p *progressReporter) update(msg string) {
	if !p.enabled {
		return
	}
	if p.isterm {
		fmt.Fprintf(p.w, "\033[2K\r%s", msg)
	} else {
		fmt.Fprintf(p.w, "%s\n", msg)
	}
	p.lastLen = len(msg)
}

// clear erases the current progress line on terminals so the final output
// starts on a clean line.
func (p *progressReporter) clear() {
	if !p.enabled || p.lastLen == 0 {
		return
	}
	if p.isterm {
		fmt.Fprintf(p.w, "\033[2K\r")
	}
	p.lastLen = 0
}
