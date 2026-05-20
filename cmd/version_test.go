package cmd

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionString_ContainsVersion(t *testing.T) {
	orig := version
	version = "1.2.3"
	defer func() { version = orig }()

	s := versionString()
	if !strings.Contains(s, "1.2.3") {
		t.Errorf("versionString() = %q, want it to contain version %q", s, "1.2.3")
	}
}

func TestVersionString_ContainsCommit(t *testing.T) {
	origV, origC := version, commit
	version = "0.5.0"
	commit = "abc1234"
	defer func() { version, commit = origV, origC }()

	s := versionString()
	if !strings.Contains(s, "abc1234") {
		t.Errorf("versionString() = %q, want it to contain commit %q", s, "abc1234")
	}
}

func TestVersionString_ContainsGoVersion(t *testing.T) {
	s := versionString()
	if !strings.Contains(s, runtime.Version()) {
		t.Errorf("versionString() = %q, want it to contain Go version %q", s, runtime.Version())
	}
}

func TestVersionString_DevDefaults(t *testing.T) {
	origV, origC, origD := version, commit, date
	version, commit, date = "dev", "unknown", "unknown"
	defer func() { version, commit, date = origV, origC, origD }()

	s := versionString()
	if !strings.HasPrefix(s, "monvif dev") {
		t.Errorf("versionString() = %q, want prefix %q", s, "monvif dev")
	}
}
