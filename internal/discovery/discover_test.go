package discovery

import (
	"testing"
)

func TestParseProbeMatch(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope"
          xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing">
  <Header/>
  <Body>
    <ProbeMatches xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery">
      <ProbeMatch>
        <XAddrs>http://192.168.1.42/onvif/device_service</XAddrs>
        <Types>dn:NetworkVideoTransmitter</Types>
        <Scopes>onvif://www.onvif.org/name/TestCamera onvif://www.onvif.org/hardware/HW1</Scopes>
      </ProbeMatch>
    </ProbeMatches>
  </Body>
</Envelope>`

	dev, err := parseProbeMatch(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dev.XAddrs) != 1 || dev.XAddrs[0] != "http://192.168.1.42/onvif/device_service" {
		t.Errorf("unexpected XAddrs: %v", dev.XAddrs)
	}
	if dev.Name != "TestCamera" {
		t.Errorf("unexpected Name: %q", dev.Name)
	}
	if len(dev.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(dev.Scopes))
	}
}

func TestParseProbeMatch_Empty(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Header/>
  <Body>
    <ProbeMatches xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery">
      <ProbeMatch>
        <XAddrs></XAddrs>
      </ProbeMatch>
    </ProbeMatches>
  </Body>
</Envelope>`

	dev, err := parseProbeMatch(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dev.XAddrs) != 0 {
		t.Errorf("expected no XAddrs, got %v", dev.XAddrs)
	}
}

func TestDefaultInterface(t *testing.T) {
	// Just verify it returns something without error on a machine with network.
	iface, err := defaultInterface()
	if err != nil {
		t.Skipf("no suitable interface found (CI environment?): %v", err)
	}
	if iface == "" {
		t.Error("expected non-empty interface name")
	}
}
