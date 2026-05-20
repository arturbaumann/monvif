package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateProgress(t *testing.T) {
	for _, valid := range []string{progressAuto, progressAlways, progressNever} {
		if err := validateProgress(valid); err != nil {
			t.Errorf("validateProgress(%q) unexpected error: %v", valid, err)
		}
	}
	for _, bad := range []string{"", "yes", "on", "Auto", "ALWAYS", "off", "1"} {
		if err := validateProgress(bad); err == nil {
			t.Errorf("validateProgress(%q) expected error, got nil", bad)
		}
	}
}

func TestResolveProgressEnabled(t *testing.T) {
	// never → always false
	if resolveProgressEnabled(progressNever, formatTable, false) {
		t.Error("progressNever should return false")
	}
	if resolveProgressEnabled(progressNever, formatTable, true) {
		t.Error("progressNever+debug should return false")
	}

	// always → true, unless debug suppresses it
	if !resolveProgressEnabled(progressAlways, formatTable, false) {
		t.Error("progressAlways should return true")
	}
	if resolveProgressEnabled(progressAlways, formatTable, true) {
		t.Error("progressAlways+debug should return false")
	}

	// auto + json → false (json output must stay clean)
	if resolveProgressEnabled(progressAuto, formatJSON, false) {
		t.Error("auto+json should return false")
	}

	// auto + debug → false
	if resolveProgressEnabled(progressAuto, formatTable, true) {
		t.Error("auto+debug should return false")
	}

	// auto + table: result depends on whether stderr is a terminal in the test
	// runner — just verify no panic and that the return type is bool.
	_ = resolveProgressEnabled(progressAuto, formatTable, false)
}

func TestProgressReporter_Disabled(t *testing.T) {
	var buf bytes.Buffer
	p := &progressReporter{w: &buf, enabled: false, isterm: false}
	p.update("should not appear")
	p.clear()
	if buf.Len() != 0 {
		t.Errorf("disabled reporter wrote %q", buf.String())
	}
}

func TestProgressReporter_NonTerminal(t *testing.T) {
	var buf bytes.Buffer
	p := &progressReporter{w: &buf, enabled: true, isterm: false}

	p.update("step one")
	p.update("step two")

	out := buf.String()
	if !strings.Contains(out, "step one\n") {
		t.Errorf("expected newline-terminated line for step one, got %q", out)
	}
	if !strings.Contains(out, "step two\n") {
		t.Errorf("expected newline-terminated line for step two, got %q", out)
	}

	// clear() must not write to a non-terminal writer
	before := buf.String()
	p.clear()
	if buf.String() != before {
		t.Errorf("clear() wrote to non-terminal: before=%q after=%q", before, buf.String())
	}
}

func TestProgressReporter_Terminal(t *testing.T) {
	var buf bytes.Buffer
	p := &progressReporter{w: &buf, enabled: true, isterm: true}

	p.update("working")
	if !strings.Contains(buf.String(), "\033[2K\r") {
		t.Errorf("expected ANSI erase+CR in terminal output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "working") {
		t.Errorf("message text missing from terminal output, got %q", buf.String())
	}

	p.clear()
	// clear() must emit an additional ANSI erase sequence
	if strings.Count(buf.String(), "\033[2K\r") < 2 {
		t.Errorf("expected clear() to emit ANSI erase, got %q", buf.String())
	}
}

func TestProgressReporter_ClearIdempotent(t *testing.T) {
	var buf bytes.Buffer
	p := &progressReporter{w: &buf, enabled: true, isterm: true}

	// clear() before any update should be a no-op (lastLen == 0)
	p.clear()
	if buf.Len() != 0 {
		t.Errorf("clear() before update wrote %q", buf.String())
	}

	p.update("msg")
	p.clear()
	buf.Reset()

	// second clear() after the first should be a no-op (lastLen reset to 0)
	p.clear()
	if buf.Len() != 0 {
		t.Errorf("second clear() wrote %q", buf.String())
	}
}
