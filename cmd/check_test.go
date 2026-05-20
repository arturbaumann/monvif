package cmd

import (
	"testing"

	"github.com/arturbaumann/monvif/internal/camera"
)

func TestToCheckJSON_Basic(t *testing.T) {
	r := camera.CheckResult{
		Name:          "front-door",
		IP:            "192.168.1.10",
		Port:          80,
		Reachable:     true,
		Authenticated: true,
		Info: camera.DeviceInfo{
			Manufacturer:    "Acme",
			Model:           "X200",
			FirmwareVersion: "2.1.0",
			SerialNumber:    "SN001",
			HardwareID:      "HW1",
		},
	}

	j := toCheckJSON(r)

	if j.Name != "front-door" {
		t.Errorf("Name: got %q", j.Name)
	}
	if j.IP != "192.168.1.10" || j.Port != 80 {
		t.Errorf("IP/Port: got %q %d", j.IP, j.Port)
	}
	if !j.Reachable || !j.Authenticated {
		t.Error("expected Reachable and Authenticated true")
	}
	if j.Manufacturer != "Acme" || j.Model != "X200" || j.Firmware != "2.1.0" {
		t.Errorf("device info mismatch: %+v", j)
	}
	if j.SerialNumber != "SN001" {
		t.Errorf("SerialNumber: got %q", j.SerialNumber)
	}
	if j.HardwareID != "HW1" {
		t.Errorf("HardwareID: got %q", j.HardwareID)
	}
	if j.Capabilities != nil {
		t.Error("expected Capabilities nil when not requested")
	}
	if j.Error != "" {
		t.Errorf("Error: got %q", j.Error)
	}
}

func TestToCheckJSON_WithCaps(t *testing.T) {
	caps := camera.Capabilities{
		DeviceXAddr:  "http://192.168.1.10/onvif/device_service",
		MediaXAddr:   "http://192.168.1.10/onvif/media",
		ImagingXAddr: "http://192.168.1.10/onvif/imaging",
		EventsXAddr:  "http://192.168.1.10/onvif/events",
		PTZXAddr:     "http://192.168.1.10/onvif/ptz",
	}
	r := camera.CheckResult{
		Name:          "garage",
		IP:            "192.168.1.11",
		Port:          80,
		Reachable:     true,
		Authenticated: true,
		Caps:          &caps,
	}

	j := toCheckJSON(r)

	if j.Capabilities == nil {
		t.Fatal("expected Capabilities to be populated")
	}
	if j.Capabilities.DeviceURL != caps.DeviceXAddr {
		t.Errorf("DeviceURL: got %q", j.Capabilities.DeviceURL)
	}
	if j.Capabilities.MediaURL != caps.MediaXAddr {
		t.Errorf("MediaURL: got %q", j.Capabilities.MediaURL)
	}
	if j.Capabilities.ImagingURL != caps.ImagingXAddr {
		t.Errorf("ImagingURL: got %q", j.Capabilities.ImagingURL)
	}
	if j.Capabilities.EventsURL != caps.EventsXAddr {
		t.Errorf("EventsURL: got %q", j.Capabilities.EventsURL)
	}
	if j.Capabilities.PTZURL != caps.PTZXAddr {
		t.Errorf("PTZURL: got %q", j.Capabilities.PTZURL)
	}
}

func TestToCheckJSON_Unreachable(t *testing.T) {
	r := camera.CheckResult{
		Name:      "offline-cam",
		IP:        "10.0.0.99",
		Port:      80,
		Reachable: false,
		Err:       "unreachable: connection refused",
	}

	j := toCheckJSON(r)

	if j.Reachable || j.Authenticated {
		t.Error("expected Reachable and Authenticated false")
	}
	if j.Error != "unreachable: connection refused" {
		t.Errorf("Error: got %q", j.Error)
	}
	if j.Capabilities != nil {
		t.Error("expected Capabilities nil on unreachable")
	}
}

func TestToCheckJSON_CapsError(t *testing.T) {
	// Device info succeeded but capabilities query failed.
	// Caps is nil; Err records the caps error; Authenticated is still true.
	r := camera.CheckResult{
		Name:          "lobby",
		IP:            "192.168.1.20",
		Port:          80,
		Reachable:     true,
		Authenticated: true,
		Info: camera.DeviceInfo{
			Manufacturer: "Vendor",
			Model:        "M1",
		},
		Caps: nil,
		Err:  "capabilities: GetCapabilities: call: some error",
	}

	j := toCheckJSON(r)

	if !j.Authenticated {
		t.Error("expected Authenticated true even when caps failed")
	}
	if j.Manufacturer != "Vendor" {
		t.Errorf("Manufacturer: got %q", j.Manufacturer)
	}
	if j.Capabilities != nil {
		t.Error("expected Capabilities nil when caps query failed")
	}
	if j.Error == "" {
		t.Error("expected Error to be set")
	}
}
