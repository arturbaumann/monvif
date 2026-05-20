package cmd

import (
	"testing"

	"github.com/artur/monvif/internal/camera"
)

func TestImagingUpdate_IsEmpty(t *testing.T) {
	var u camera.ImagingUpdate
	if !u.IsEmpty() {
		t.Error("zero ImagingUpdate should be empty")
	}
	v := 1.0
	u.Brightness = &v
	if u.IsEmpty() {
		t.Error("ImagingUpdate with Brightness set should not be empty")
	}
}

func TestValidateImagingSetArgs_Empty(t *testing.T) {
	err := validateImagingSetArgs(camera.ImagingUpdate{}, false, false)
	if err == nil {
		t.Error("expected error for empty update")
	}
}

func TestValidateImagingSetArgs_NoConfirmation(t *testing.T) {
	v := 50.0
	u := camera.ImagingUpdate{Brightness: &v}
	err := validateImagingSetArgs(u, false, false)
	if err == nil {
		t.Error("expected error when neither --yes nor --dry-run given")
	}
}

func TestValidateImagingSetArgs_DryRunOK(t *testing.T) {
	v := 50.0
	u := camera.ImagingUpdate{Brightness: &v}
	if err := validateImagingSetArgs(u, false, true); err != nil {
		t.Errorf("unexpected error for --dry-run: %v", err)
	}
}

func TestValidateImagingSetArgs_YesOK(t *testing.T) {
	v := 50.0
	u := camera.ImagingUpdate{Brightness: &v}
	if err := validateImagingSetArgs(u, true, false); err != nil {
		t.Errorf("unexpected error for --yes: %v", err)
	}
}
