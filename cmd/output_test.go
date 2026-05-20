package cmd

import (
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
