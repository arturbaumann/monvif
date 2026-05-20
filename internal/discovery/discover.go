package discovery

import (
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/beevik/etree"
	wsdiscovery "github.com/use-go/onvif/ws-discovery"
)

// Device holds the information parsed from a WS-Discovery ProbeMatch response.
type Device struct {
	XAddrs []string
	Scopes []string
	Name   string
	Types  string
}

// Discover sends a WS-Discovery probe on the given interface and returns parsed devices.
// timeout controls how long to wait; the underlying library uses a fixed 1 s read deadline,
// so we loop until timeout is exhausted to collect late responders.
func Discover(iface string, timeout time.Duration) ([]Device, error) {
	if iface == "" {
		detected, err := defaultInterface()
		if err != nil {
			return nil, fmt.Errorf("no interface specified and could not detect one: %w", err)
		}
		iface = detected
	}

	// The library's SendProbe uses a 1 s read deadline. We loop until timeout.
	deadline := time.Now().Add(timeout)
	seen := map[string]bool{}
	var devices []Device

	for time.Now().Before(deadline) {
		raw, err := wsdiscovery.SendProbe(
			iface,
			nil,
			[]string{"dn:NetworkVideoTransmitter"},
			map[string]string{"dn": "http://www.onvif.org/ver10/network/wsdl"},
		)
		if err != nil {
			// A deadline error from the library just means no more responses in this round.
			break
		}
		for _, msg := range raw {
			dev, err := parseProbeMatch(msg)
			if err != nil || len(dev.XAddrs) == 0 {
				continue
			}
			key := strings.Join(dev.XAddrs, ",")
			if !seen[key] {
				seen[key] = true
				devices = append(devices, dev)
			}
		}
	}

	return devices, nil
}

func parseProbeMatch(xmlStr string) (Device, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		return Device{}, err
	}

	var dev Device

	for _, el := range doc.Root().FindElements("./Body/ProbeMatches/ProbeMatch/XAddrs") {
		for _, addr := range strings.Fields(el.Text()) {
			dev.XAddrs = append(dev.XAddrs, addr)
		}
	}

	for _, el := range doc.Root().FindElements("./Body/ProbeMatches/ProbeMatch/Types") {
		dev.Types = el.Text()
	}

	for _, el := range doc.Root().FindElements("./Body/ProbeMatches/ProbeMatch/Scopes") {
		for _, scope := range strings.Fields(el.Text()) {
			dev.Scopes = append(dev.Scopes, scope)
		}
	}

	re := regexp.MustCompile(`onvif://www\.onvif\.org/name/([^/\s]+)`)
	for _, scope := range dev.Scopes {
		if m := re.FindStringSubmatch(scope); len(m) > 1 {
			dev.Name = path.Base(m[1])
			break
		}
	}

	return dev, nil
}

// defaultInterface returns the first non-loopback interface with an IPv4 address.
func defaultInterface() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}
