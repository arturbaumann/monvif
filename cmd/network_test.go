package cmd

import (
	"testing"
)

// ── set-ip validation ─────────────────────────────────────────────────────────

func TestValidateSetIPArgs_DHCPValid(t *testing.T) {
	if err := validateSetIPArgs("eth0", "", "", 0, true, true, false); err != nil {
		t.Errorf("unexpected error for DHCP dry-run: %v", err)
	}
}

func TestValidateSetIPArgs_StaticValid(t *testing.T) {
	if err := validateSetIPArgs("eth0", "192.168.1.10", "192.168.1.1", 24, false, true, false); err != nil {
		t.Errorf("unexpected error for static dry-run: %v", err)
	}
}

func TestValidateSetIPArgs_DHCPAndAddress(t *testing.T) {
	err := validateSetIPArgs("eth0", "192.168.1.10", "", 24, true, true, false)
	if err == nil {
		t.Error("expected error for --dhcp + --address")
	}
}

func TestValidateSetIPArgs_MissingInterface(t *testing.T) {
	// --interface is required by cobra; validateSetIPArgs is only called if it's set.
	// The cobra Required constraint handles this case before RunE is called.
	// Included here to document behavior — no extra validation in the function.
	_ = "interface is required by cobra MarkFlagRequired"
}

func TestValidateSetIPArgs_NoModeSpecified(t *testing.T) {
	err := validateSetIPArgs("eth0", "", "", 0, false, true, false)
	if err == nil {
		t.Error("expected error when neither --dhcp nor --address given")
	}
}

func TestValidateSetIPArgs_InvalidIP(t *testing.T) {
	err := validateSetIPArgs("eth0", "not-an-ip", "", 24, false, true, false)
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestValidateSetIPArgs_InvalidPrefix(t *testing.T) {
	for _, bad := range []int{0, 33, -1, 99} {
		err := validateSetIPArgs("eth0", "192.168.1.10", "", bad, false, true, false)
		if err == nil {
			t.Errorf("expected error for prefix-length %d", bad)
		}
	}
}

func TestValidateSetIPArgs_NoConfirmation(t *testing.T) {
	err := validateSetIPArgs("eth0", "192.168.1.10", "", 24, false, false, false)
	if err == nil {
		t.Error("expected error when neither --dry-run nor --yes given")
	}
}

func TestValidateSetIPArgs_DryRunAndYes(t *testing.T) {
	err := validateSetIPArgs("eth0", "192.168.1.10", "", 24, false, true, true)
	if err == nil {
		t.Error("expected error for --dry-run + --yes")
	}
}

func TestValidateSetIPArgs_YesValid(t *testing.T) {
	if err := validateSetIPArgs("eth0", "192.168.1.10", "", 24, false, false, true); err != nil {
		t.Errorf("unexpected error for --yes: %v", err)
	}
}

func TestValidateSetIPArgs_InvalidGateway(t *testing.T) {
	err := validateSetIPArgs("eth0", "192.168.1.10", "not-a-gw", 24, false, true, false)
	if err == nil {
		t.Error("expected error for invalid gateway")
	}
}

// ── set-dns validation ────────────────────────────────────────────────────────

func TestValidateSetDNSArgs_DHCPValid(t *testing.T) {
	if err := validateSetDNSArgs(true, nil, true, false); err != nil {
		t.Errorf("unexpected error for DNS DHCP dry-run: %v", err)
	}
}

func TestValidateSetDNSArgs_ManualValid(t *testing.T) {
	if err := validateSetDNSArgs(false, []string{"8.8.8.8"}, true, false); err != nil {
		t.Errorf("unexpected error for manual DNS dry-run: %v", err)
	}
}

func TestValidateSetDNSArgs_DHCPAndServer(t *testing.T) {
	err := validateSetDNSArgs(true, []string{"8.8.8.8"}, true, false)
	if err == nil {
		t.Error("expected error for --dhcp + --server")
	}
}

func TestValidateSetDNSArgs_InvalidServer(t *testing.T) {
	err := validateSetDNSArgs(false, []string{"not-an-ip"}, true, false)
	if err == nil {
		t.Error("expected error for invalid DNS server")
	}
}

func TestValidateSetDNSArgs_NoConfirmation(t *testing.T) {
	err := validateSetDNSArgs(false, []string{"8.8.8.8"}, false, false)
	if err == nil {
		t.Error("expected error when neither --dry-run nor --yes")
	}
}

func TestValidateSetDNSArgs_NeitherMode(t *testing.T) {
	err := validateSetDNSArgs(false, nil, true, false)
	if err == nil {
		t.Error("expected error when neither --dhcp nor --server given")
	}
}

// ── set-ntp validation ────────────────────────────────────────────────────────

func TestValidateSetNTPArgs_DHCPValid(t *testing.T) {
	if err := validateSetNTPArgs(true, nil, true, false); err != nil {
		t.Errorf("unexpected error for NTP DHCP dry-run: %v", err)
	}
}

func TestValidateSetNTPArgs_ManualIPValid(t *testing.T) {
	if err := validateSetNTPArgs(false, []string{"192.168.1.1"}, true, false); err != nil {
		t.Errorf("unexpected error for manual NTP IP: %v", err)
	}
}

func TestValidateSetNTPArgs_ManualHostnameValid(t *testing.T) {
	if err := validateSetNTPArgs(false, []string{"se.pool.ntp.org"}, true, false); err != nil {
		t.Errorf("unexpected error for NTP hostname: %v", err)
	}
}

func TestValidateSetNTPArgs_DHCPAndServer(t *testing.T) {
	err := validateSetNTPArgs(true, []string{"pool.ntp.org"}, true, false)
	if err == nil {
		t.Error("expected error for --dhcp + --server")
	}
}

func TestValidateSetNTPArgs_NeitherMode(t *testing.T) {
	err := validateSetNTPArgs(false, nil, true, false)
	if err == nil {
		t.Error("expected error when neither --dhcp nor --server given")
	}
}

func TestValidateSetNTPArgs_NoConfirmation(t *testing.T) {
	err := validateSetNTPArgs(false, []string{"pool.ntp.org"}, false, false)
	if err == nil {
		t.Error("expected error when neither --dry-run nor --yes")
	}
}

// ── set-hostname validation ───────────────────────────────────────────────────

func TestValidateSetHostnameArgs_Valid(t *testing.T) {
	for _, name := range []string{"camera1", "front-door", "Camera01", "a"} {
		if err := validateSetHostnameArgs(name, true, false); err != nil {
			t.Errorf("validateSetHostnameArgs(%q) unexpected error: %v", name, err)
		}
	}
}

func TestValidateSetHostnameArgs_InvalidChars(t *testing.T) {
	for _, name := range []string{"cam era", "cam_era", "cam.era", "-camera", "camera-"} {
		if err := validateSetHostnameArgs(name, true, false); err == nil {
			t.Errorf("validateSetHostnameArgs(%q) expected error, got nil", name)
		}
	}
}

func TestValidateSetHostnameArgs_TooLong(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01"
	if err := validateSetHostnameArgs(long, true, false); err == nil {
		t.Error("expected error for hostname longer than 63 chars")
	}
}

func TestValidateSetHostnameArgs_NoConfirmation(t *testing.T) {
	if err := validateSetHostnameArgs("camera", false, false); err == nil {
		t.Error("expected error when neither --dry-run nor --yes")
	}
}

func TestValidateSetHostnameArgs_DryRunAndYes(t *testing.T) {
	if err := validateSetHostnameArgs("camera", true, true); err == nil {
		t.Error("expected error for --dry-run + --yes")
	}
}

// ── network protocols rendering ───────────────────────────────────────────────

func TestPortList_Empty(t *testing.T) {
	if got := portList(nil); got != "" {
		t.Errorf("portList(nil) = %q, want empty string", got)
	}
	if got := portList([]int{}); got != "" {
		t.Errorf("portList([]) = %q, want empty string", got)
	}
}

func TestPortList_Single(t *testing.T) {
	if got := portList([]int{80}); got != "80" {
		t.Errorf("portList([80]) = %q, want \"80\"", got)
	}
}

func TestPortList_Multiple(t *testing.T) {
	if got := portList([]int{80, 443, 554}); got != "80,443,554" {
		t.Errorf("portList([80,443,554]) = %q, want \"80,443,554\"", got)
	}
}
