package camera

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/use-go/onvif/device"
	"github.com/use-go/onvif/sdk"
	sdkdevice "github.com/use-go/onvif/sdk/device"
	"github.com/use-go/onvif/xsd"
	onvifxsd "github.com/use-go/onvif/xsd/onvif"
)

// ── read types ────────────────────────────────────────────────────────────────

// PrefixedIPv4 represents an IPv4 address with a CIDR prefix length.
type PrefixedIPv4 struct {
	Address      string `json:"address"`
	PrefixLength int    `json:"prefix_length"`
}

func (p PrefixedIPv4) String() string {
	if p.Address == "" {
		return ""
	}
	return fmt.Sprintf("%s/%d", p.Address, p.PrefixLength)
}

// NetworkInterfaceInfo holds a camera network interface state.
type NetworkInterfaceInfo struct {
	Token         string         `json:"token"`
	Enabled       bool           `json:"enabled"`
	Name          string         `json:"name,omitempty"`
	HwAddress     string         `json:"hw_address,omitempty"`
	MTU           int            `json:"mtu,omitempty"`
	IPv4Enabled   bool           `json:"ipv4_enabled"`
	IPv4DHCP      bool           `json:"ipv4_dhcp"`
	IPv4Manual    []PrefixedIPv4 `json:"ipv4_manual,omitempty"`
	IPv4FromDHCP  *PrefixedIPv4  `json:"ipv4_from_dhcp,omitempty"`
	IPv4LinkLocal *PrefixedIPv4  `json:"ipv4_link_local,omitempty"`
	IPv6Enabled   bool           `json:"ipv6_enabled"`
}

// NetworkProtocolInfo holds a camera network protocol state.
type NetworkProtocolInfo struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Ports   []int  `json:"ports,omitempty"`
}

// DNSInfo holds camera DNS configuration.
type DNSInfo struct {
	FromDHCP      bool     `json:"from_dhcp"`
	SearchDomain  string   `json:"search_domain,omitempty"`
	FromDHCPAddrs []string `json:"from_dhcp_addresses,omitempty"`
	ManualAddrs   []string `json:"manual_addresses,omitempty"`
}

// NTPInfo holds camera NTP configuration.
type NTPInfo struct {
	FromDHCP    bool     `json:"from_dhcp"`
	Manual      []string `json:"manual,omitempty"`
	FromDHCPSrv []string `json:"from_dhcp_servers,omitempty"`
}

// HostnameInfo holds camera hostname configuration.
type HostnameInfo struct {
	FromDHCP bool   `json:"from_dhcp"`
	Name     string `json:"name,omitempty"`
}

// NetworkSummary collects all network settings from multiple ONVIF calls.
// Partial failures are accumulated in Warnings rather than aborting.
type NetworkSummary struct {
	Hostname   *HostnameInfo          `json:"hostname,omitempty"`
	Interfaces []NetworkInterfaceInfo `json:"interfaces,omitempty"`
	Gateway    []string               `json:"gateway,omitempty"`
	DNS        *DNSInfo               `json:"dns,omitempty"`
	NTP        *NTPInfo               `json:"ntp,omitempty"`
	Protocols  []NetworkProtocolInfo  `json:"protocols,omitempty"`
	Warnings   []string               `json:"warnings,omitempty"`
}

// ── write types ───────────────────────────────────────────────────────────────

// SetIPConfig describes a requested IPv4 change.
type SetIPConfig struct {
	InterfaceToken string
	DHCP           bool
	Address        string
	PrefixLength   int
	Gateway        string // empty = do not change gateway
}

// SetIPResult reports the outcome of a SetNetworkInterfaces call.
type SetIPResult struct {
	RebootNeeded bool
}

// SetDNSConfig describes a requested DNS change.
type SetDNSConfig struct {
	FromDHCP bool
	Servers  []string
}

// SetNTPConfig describes a requested NTP change.
type SetNTPConfig struct {
	FromDHCP bool
	Servers  []string // IP addresses or hostnames
}

// ── read operations ───────────────────────────────────────────────────────────

// GetNetworkInterfaces returns all network interfaces reported by the camera.
// It uses a hand-written envelope to collect multiple interfaces, working
// around the library's singular response struct.
func (c *Client) GetNetworkInterfaces(ctx context.Context) ([]NetworkInterfaceInfo, error) {
	type ifaceResponse struct {
		NetworkInterfaces []onvifxsd.NetworkInterface
	}
	type envelope struct {
		Header struct{}
		Body   struct {
			GetNetworkInterfacesResponse ifaceResponse
		}
	}
	httpResp, err := c.dev.CallMethod(device.GetNetworkInterfaces{})
	if err != nil {
		return nil, fmt.Errorf("GetNetworkInterfaces: %w", err)
	}
	var reply envelope
	if err := sdk.ReadAndParse(ctx, httpResp, &reply, "GetNetworkInterfaces"); err != nil {
		return nil, err
	}
	var result []NetworkInterfaceInfo
	for _, raw := range reply.Body.GetNetworkInterfacesResponse.NetworkInterfaces {
		result = append(result, rawToNetworkInterface(raw))
	}
	return result, nil
}

// GetNetworkProtocols returns all network protocols reported by the camera.
// Uses a hand-written envelope and a custom parse struct to avoid the
// xml:"onvif:Name" tag mismatch in onvifxsd.NetworkProtocol — Go's XML
// decoder treats "onvif:Name" as a literal element name, which never matches
// real camera responses that use namespace-prefixed elements like <tt:Name>.
func (c *Client) GetNetworkProtocols(ctx context.Context) ([]NetworkProtocolInfo, error) {
	// rawProto uses unqualified xml tags so Go XML matches by local name,
	// regardless of the namespace prefix the camera uses.
	type rawProto struct {
		Name    string `xml:"Name"`
		Enabled bool   `xml:"Enabled"`
		Port    int32  `xml:"Port"`
	}
	type envelope struct {
		Header struct{}
		Body   struct {
			GetNetworkProtocolsResponse struct {
				NetworkProtocols []rawProto
			}
		}
	}
	httpResp, err := c.dev.CallMethod(device.GetNetworkProtocols{})
	if err != nil {
		return nil, fmt.Errorf("GetNetworkProtocols: %w", err)
	}
	var reply envelope
	if err := sdk.ReadAndParse(ctx, httpResp, &reply, "GetNetworkProtocols"); err != nil {
		return nil, err
	}
	raw := reply.Body.GetNetworkProtocolsResponse.NetworkProtocols
	log.Debug().Int("count", len(raw)).Msg("GetNetworkProtocols: raw entries")
	var result []NetworkProtocolInfo
	for _, r := range raw {
		log.Debug().Str("name", r.Name).Bool("enabled", r.Enabled).Int32("port", r.Port).Msg("GetNetworkProtocols: entry")
		if r.Name == "" {
			continue
		}
		p := NetworkProtocolInfo{
			Name:    strings.TrimSpace(r.Name),
			Enabled: r.Enabled,
		}
		if r.Port > 0 {
			p.Ports = []int{int(r.Port)}
		}
		result = append(result, p)
	}
	return result, nil
}

// GetDNSInfo returns the camera DNS configuration.
func (c *Client) GetDNSInfo(ctx context.Context) (DNSInfo, error) {
	resp, err := sdkdevice.Call_GetDNS(ctx, c.dev, device.GetDNS{})
	if err != nil {
		return DNSInfo{}, fmt.Errorf("GetDNS: %w", err)
	}
	return rawToDNSInfo(resp.DNSInformation), nil
}

// GetNTPInfo returns the camera NTP configuration.
func (c *Client) GetNTPInfo(ctx context.Context) (NTPInfo, error) {
	resp, err := sdkdevice.Call_GetNTP(ctx, c.dev, device.GetNTP{})
	if err != nil {
		return NTPInfo{}, fmt.Errorf("GetNTP: %w", err)
	}
	return rawToNTPInfo(resp.NTPInformation), nil
}

// GetHostnameInfo returns the camera hostname configuration.
func (c *Client) GetHostnameInfo(ctx context.Context) (HostnameInfo, error) {
	resp, err := sdkdevice.Call_GetHostname(ctx, c.dev, device.GetHostname{})
	if err != nil {
		return HostnameInfo{}, fmt.Errorf("GetHostname: %w", err)
	}
	return HostnameInfo{
		FromDHCP: bool(resp.HostnameInformation.FromDHCP),
		Name:     strings.TrimSpace(string(resp.HostnameInformation.Name)),
	}, nil
}

// GetNetworkGateway returns configured gateway IPv4 addresses.
func (c *Client) GetNetworkGateway(ctx context.Context) ([]string, error) {
	resp, err := sdkdevice.Call_GetNetworkDefaultGateway(ctx, c.dev, device.GetNetworkDefaultGateway{})
	if err != nil {
		return nil, fmt.Errorf("GetNetworkDefaultGateway: %w", err)
	}
	var gw []string
	if v := strings.TrimSpace(string(resp.NetworkGateway.IPv4Address)); v != "" {
		gw = append(gw, v)
	}
	if v := strings.TrimSpace(string(resp.NetworkGateway.IPv6Address)); v != "" {
		gw = append(gw, v)
	}
	return gw, nil
}

// GetNetworkSummary assembles all network info, tolerating partial failures.
func (c *Client) GetNetworkSummary(ctx context.Context) NetworkSummary {
	var s NetworkSummary

	if ifaces, err := c.GetNetworkInterfaces(ctx); err != nil {
		s.Warnings = append(s.Warnings, "interfaces: "+err.Error())
	} else {
		s.Interfaces = ifaces
	}

	if gw, err := c.GetNetworkGateway(ctx); err != nil {
		s.Warnings = append(s.Warnings, "gateway: "+err.Error())
	} else {
		s.Gateway = gw
	}

	if dns, err := c.GetDNSInfo(ctx); err != nil {
		s.Warnings = append(s.Warnings, "dns: "+err.Error())
	} else {
		s.DNS = &dns
	}

	if ntp, err := c.GetNTPInfo(ctx); err != nil {
		s.Warnings = append(s.Warnings, "ntp: "+err.Error())
	} else {
		s.NTP = &ntp
	}

	if h, err := c.GetHostnameInfo(ctx); err != nil {
		s.Warnings = append(s.Warnings, "hostname: "+err.Error())
	} else {
		s.Hostname = &h
	}

	if protos, err := c.GetNetworkProtocols(ctx); err != nil {
		s.Warnings = append(s.Warnings, "protocols: "+err.Error())
	} else {
		s.Protocols = protos
	}

	return s
}

// ── write operations ──────────────────────────────────────────────────────────

// SetNetworkIP applies an IPv4 configuration change via SetNetworkInterfaces,
// and optionally updates the default gateway via SetNetworkDefaultGateway.
func (c *Client) SetNetworkIP(ctx context.Context, cfg SetIPConfig) (SetIPResult, error) {
	netIface := onvifxsd.NetworkInterfaceSetConfiguration{
		Enabled: xsd.Boolean(true),
		IPv4: onvifxsd.IPv4NetworkInterfaceSetConfiguration{
			Enabled: xsd.Boolean(true),
			DHCP:    xsd.Boolean(cfg.DHCP),
		},
	}
	if !cfg.DHCP {
		netIface.IPv4.Manual = onvifxsd.PrefixedIPv4Address{
			Address:      onvifxsd.IPv4Address(cfg.Address),
			PrefixLength: xsd.Int(cfg.PrefixLength),
		}
	}

	resp, err := sdkdevice.Call_SetNetworkInterfaces(ctx, c.dev, device.SetNetworkInterfaces{
		InterfaceToken:   onvifxsd.ReferenceToken(cfg.InterfaceToken),
		NetworkInterface: netIface,
	})
	if err != nil {
		return SetIPResult{}, fmt.Errorf("SetNetworkInterfaces: %w", err)
	}

	result := SetIPResult{RebootNeeded: bool(resp.RebootNeeded)}

	if cfg.Gateway != "" {
		if _, err := sdkdevice.Call_SetNetworkDefaultGateway(ctx, c.dev, device.SetNetworkDefaultGateway{
			IPv4Address: onvifxsd.IPv4Address(cfg.Gateway),
		}); err != nil {
			return result, fmt.Errorf("SetNetworkDefaultGateway: %w", err)
		}
	}

	return result, nil
}

// SetDNS applies a DNS configuration change.
// Due to a library limitation, only the first manual server is sent to the camera.
func (c *Client) SetDNS(ctx context.Context, cfg SetDNSConfig) error {
	req := device.SetDNS{
		FromDHCP: xsd.Boolean(cfg.FromDHCP),
	}
	if !cfg.FromDHCP && len(cfg.Servers) > 0 {
		req.DNSManual = onvifxsd.IPAddress{
			Type:        onvifxsd.IPType("IPv4"),
			IPv4Address: onvifxsd.IPv4Address(cfg.Servers[0]),
		}
	}
	if _, err := sdkdevice.Call_SetDNS(ctx, c.dev, req); err != nil {
		return fmt.Errorf("SetDNS: %w", err)
	}
	return nil
}

// SetNTP applies an NTP configuration change.
// Due to a library limitation, only the first manual server is sent to the camera.
func (c *Client) SetNTP(ctx context.Context, cfg SetNTPConfig) error {
	req := device.SetNTP{
		FromDHCP: xsd.Boolean(cfg.FromDHCP),
	}
	if !cfg.FromDHCP && len(cfg.Servers) > 0 {
		srv := cfg.Servers[0]
		if net.ParseIP(srv) != nil {
			req.NTPManual = onvifxsd.NetworkHost{
				Type:        onvifxsd.NetworkHostType("IPv4"),
				IPv4Address: onvifxsd.IPv4Address(srv),
			}
		} else {
			req.NTPManual = onvifxsd.NetworkHost{
				Type:    onvifxsd.NetworkHostType("DNS"),
				DNSname: onvifxsd.DNSName(srv),
			}
		}
	}
	if _, err := sdkdevice.Call_SetNTP(ctx, c.dev, req); err != nil {
		return fmt.Errorf("SetNTP: %w", err)
	}
	return nil
}

// SetHostname applies a hostname change.
func (c *Client) SetHostname(ctx context.Context, name string) error {
	if _, err := sdkdevice.Call_SetHostname(ctx, c.dev, device.SetHostname{
		Name: xsd.Token(name),
	}); err != nil {
		return fmt.Errorf("SetHostname: %w", err)
	}
	return nil
}

// ── internal parsers ──────────────────────────────────────────────────────────

func rawToNetworkInterface(raw onvifxsd.NetworkInterface) NetworkInterfaceInfo {
	iface := NetworkInterfaceInfo{
		Token:       strings.TrimSpace(string(raw.Token)),
		Enabled:     bool(raw.Enabled),
		Name:        strings.TrimSpace(string(raw.Info.Name)),
		HwAddress:   strings.TrimSpace(string(raw.Info.HwAddress)),
		MTU:         int(raw.Info.MTU),
		IPv4Enabled: bool(raw.IPv4.Enabled),
		IPv4DHCP:    bool(raw.IPv4.Config.DHCP),
		IPv6Enabled: bool(raw.IPv6.Enabled),
	}
	if addr := strings.TrimSpace(string(raw.IPv4.Config.Manual.Address)); addr != "" {
		iface.IPv4Manual = []PrefixedIPv4{{
			Address:      addr,
			PrefixLength: int(raw.IPv4.Config.Manual.PrefixLength),
		}}
	}
	if addr := strings.TrimSpace(string(raw.IPv4.Config.FromDHCP.Address)); addr != "" {
		v := PrefixedIPv4{Address: addr, PrefixLength: int(raw.IPv4.Config.FromDHCP.PrefixLength)}
		iface.IPv4FromDHCP = &v
	}
	if addr := strings.TrimSpace(string(raw.IPv4.Config.LinkLocal.Address)); addr != "" {
		v := PrefixedIPv4{Address: addr, PrefixLength: int(raw.IPv4.Config.LinkLocal.PrefixLength)}
		iface.IPv4LinkLocal = &v
	}
	return iface
}

func rawToDNSInfo(raw onvifxsd.DNSInformation) DNSInfo {
	d := DNSInfo{
		FromDHCP:     bool(raw.FromDHCP),
		SearchDomain: strings.TrimSpace(string(raw.SearchDomain)),
	}
	if addr := strings.TrimSpace(string(raw.DNSFromDHCP.IPv4Address)); addr != "" {
		d.FromDHCPAddrs = append(d.FromDHCPAddrs, addr)
	}
	if addr := strings.TrimSpace(string(raw.DNSFromDHCP.IPv6Address)); addr != "" {
		d.FromDHCPAddrs = append(d.FromDHCPAddrs, addr)
	}
	if addr := strings.TrimSpace(string(raw.DNSManual.IPv4Address)); addr != "" {
		d.ManualAddrs = append(d.ManualAddrs, addr)
	}
	if addr := strings.TrimSpace(string(raw.DNSManual.IPv6Address)); addr != "" {
		d.ManualAddrs = append(d.ManualAddrs, addr)
	}
	return d
}

func rawToNTPInfo(raw onvifxsd.NTPInformation) NTPInfo {
	n := NTPInfo{FromDHCP: bool(raw.FromDHCP)}
	if v := ntpHostStr(raw.NTPFromDHCP); v != "" {
		n.FromDHCPSrv = []string{v}
	}
	if v := ntpHostStr(raw.NTPManual); v != "" {
		n.Manual = []string{v}
	}
	return n
}

func ntpHostStr(h onvifxsd.NetworkHost) string {
	if v := strings.TrimSpace(string(h.DNSname)); v != "" {
		return v
	}
	if v := strings.TrimSpace(string(h.IPv4Address)); v != "" {
		return v
	}
	if v := strings.TrimSpace(string(h.IPv6Address)); v != "" {
		return v
	}
	return ""
}
