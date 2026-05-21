package cmd

import (
	"strings"
	"testing"
)

func TestValidateFormat(t *testing.T) {
	for _, f := range []string{formatTable, formatJSON} {
		if err := validateFormat(f); err != nil {
			t.Errorf("validateFormat(%q) unexpected error: %v", f, err)
		}
	}
	for _, bad := range []string{"", "csv", "yaml", "TABLE", "JSON"} {
		if err := validateFormat(bad); err == nil {
			t.Errorf("validateFormat(%q) expected error, got nil", bad)
		}
	}
}

func TestBoolMark(t *testing.T) {
	if boolMark(true) != "yes" {
		t.Error("boolMark(true) != yes")
	}
	if boolMark(false) != "no" {
		t.Error("boolMark(false) != no")
	}
}

func TestRedactPassword_WithPassword(t *testing.T) {
	raw := "rtsp://ha:testpass@192.0.2.1:554/1/1"
	got := redactPassword(raw)
	if strings.Contains(got, "testpass") {
		t.Errorf("password must be redacted: %q", got)
	}
	if !strings.Contains(got, "ha:") {
		t.Errorf("username must remain visible: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker: %q", got)
	}
}

func TestRedactPassword_NoUserinfo(t *testing.T) {
	raw := "rtsp://192.0.2.1:554/1/1"
	got := redactPassword(raw)
	if got != raw {
		t.Errorf("URI without userinfo must be returned unchanged: got %q", got)
	}
}

func TestRedactPassword_UsernameOnly(t *testing.T) {
	// No password field — must be returned unchanged.
	raw := "rtsp://user@host/stream"
	got := redactPassword(raw)
	if got != raw {
		t.Errorf("URI with username but no password must be returned unchanged: got %q", got)
	}
}

func TestRedactPassword_ParseFailure(t *testing.T) {
	// Unparseable URI must be returned unchanged without panic.
	raw := "://bad uri"
	got := redactPassword(raw)
	if got != raw {
		t.Errorf("unparseable URI must be returned unchanged: got %q", got)
	}
}
