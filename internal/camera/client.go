package camera

import (
	"context"
	"fmt"
	"strings"

	"github.com/use-go/onvif"
	"github.com/use-go/onvif/device"
	"github.com/use-go/onvif/media"
	sdkdevice "github.com/use-go/onvif/sdk/device"
	sdkmedia "github.com/use-go/onvif/sdk/media"
	onvifxsd "github.com/use-go/onvif/xsd/onvif"
)

// Client wraps an ONVIF device connection.
type Client struct {
	dev *onvif.Device
}

// New creates a new Client connected to the given host:port.
func New(host string, port int, username, password string) (*Client, error) {
	xaddr := fmt.Sprintf("%s:%d", host, port)
	dev, err := onvif.NewDevice(onvif.DeviceParams{
		Xaddr:    xaddr,
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", xaddr, err)
	}
	return &Client{dev: dev}, nil
}

// DeviceInfo holds basic device information.
type DeviceInfo struct {
	Manufacturer    string
	Model           string
	FirmwareVersion string
	SerialNumber    string
	HardwareID      string
}

// GetDeviceInfo queries the device service for basic information.
func (c *Client) GetDeviceInfo(ctx context.Context) (DeviceInfo, error) {
	resp, err := sdkdevice.Call_GetDeviceInformation(ctx, c.dev, device.GetDeviceInformation{})
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("GetDeviceInformation: %w", err)
	}
	return DeviceInfo{
		Manufacturer:    string(resp.Manufacturer),
		Model:           string(resp.Model),
		FirmwareVersion: string(resp.FirmwareVersion),
		SerialNumber:    string(resp.SerialNumber),
		HardwareID:      string(resp.HardwareId),
	}, nil
}

// Capabilities holds the service URLs reported by the device.
type Capabilities struct {
	DeviceXAddr  string
	MediaXAddr   string
	ImagingXAddr string
	EventsXAddr  string
	PTZXAddr     string
}

// GetCapabilities queries service capability URLs.
func (c *Client) GetCapabilities(ctx context.Context) (Capabilities, error) {
	resp, err := sdkdevice.Call_GetCapabilities(ctx, c.dev, device.GetCapabilities{
		Category: onvifxsd.CapabilityCategory("All"),
	})
	if err != nil {
		return Capabilities{}, fmt.Errorf("GetCapabilities: %w", err)
	}
	caps := resp.Capabilities
	return Capabilities{
		DeviceXAddr:  string(caps.Device.XAddr),
		MediaXAddr:   string(caps.Media.XAddr),
		ImagingXAddr: string(caps.Imaging.XAddr),
		EventsXAddr:  string(caps.Events.XAddr),
		PTZXAddr:     string(caps.PTZ.XAddr),
	}, nil
}

// Profile holds a media profile token and name.
type Profile struct {
	Token string
	Name  string
}

// GetProfiles returns available media profiles.
func (c *Client) GetProfiles(ctx context.Context) ([]Profile, error) {
	resp, err := sdkmedia.Call_GetProfiles(ctx, c.dev, media.GetProfiles{})
	if err != nil {
		return nil, fmt.Errorf("GetProfiles: %w", err)
	}
	var profiles []Profile
	for _, p := range resp.Profiles {
		profiles = append(profiles, Profile{
			Token: string(p.Token),
			Name:  string(p.Name),
		})
	}
	return profiles, nil
}

// StreamURIOptions configures stream URI transport.
type StreamURIOptions struct {
	Transport string // rtsp (default), tcp, http, udp
}

// GetStreamURI returns the stream URI for the given profile token.
// If token is empty, the first available profile is used.
// Returns profileToken, profileName, uri.
func (c *Client) GetStreamURI(ctx context.Context, token string, opts StreamURIOptions) (profileToken, profileName, uri string, err error) {
	profileToken, profileName, err = c.resolveProfile(ctx, token)
	if err != nil {
		return
	}
	protocol := streamTransportProtocol(opts.Transport)
	resp, err := sdkmedia.Call_GetStreamUri(ctx, c.dev, media.GetStreamUri{
		ProfileToken: onvifxsd.ReferenceToken(profileToken),
		StreamSetup: onvifxsd.StreamSetup{
			Stream:    onvifxsd.StreamType("RTP-Unicast"),
			Transport: onvifxsd.Transport{Protocol: protocol},
		},
	})
	if err != nil {
		err = fmt.Errorf("GetStreamUri: %w", err)
		return
	}
	uri = strings.TrimSpace(string(resp.MediaUri.Uri))
	return
}

// GetStreamURIForToken returns the RTSP stream URI for a known profile token
// without calling GetProfiles. Use this when the caller already holds the
// profile list to avoid a redundant ONVIF round-trip per profile.
func (c *Client) GetStreamURIForToken(ctx context.Context, token string, opts StreamURIOptions) (string, error) {
	protocol := streamTransportProtocol(opts.Transport)
	resp, err := sdkmedia.Call_GetStreamUri(ctx, c.dev, media.GetStreamUri{
		ProfileToken: onvifxsd.ReferenceToken(token),
		StreamSetup: onvifxsd.StreamSetup{
			Stream:    onvifxsd.StreamType("RTP-Unicast"),
			Transport: onvifxsd.Transport{Protocol: protocol},
		},
	})
	if err != nil {
		return "", fmt.Errorf("GetStreamUri: %w", err)
	}
	return strings.TrimSpace(string(resp.MediaUri.Uri)), nil
}

// GetSnapshotURI returns the snapshot URI for the given profile token.
// If token is empty, the first available profile is used.
// Returns profileToken, profileName, uri.
func (c *Client) GetSnapshotURI(ctx context.Context, token string) (profileToken, profileName, uri string, err error) {
	profileToken, profileName, err = c.resolveProfile(ctx, token)
	if err != nil {
		return
	}
	resp, err := sdkmedia.Call_GetSnapshotUri(ctx, c.dev, media.GetSnapshotUri{
		ProfileToken: onvifxsd.ReferenceToken(profileToken),
	})
	if err != nil {
		err = fmt.Errorf("GetSnapshotUri: %w", err)
		return
	}
	uri = strings.TrimSpace(string(resp.MediaUri.Uri))
	return
}

func (c *Client) resolveProfile(ctx context.Context, token string) (resolvedToken, profileName string, err error) {
	profiles, err := c.GetProfiles(ctx)
	if err != nil {
		return
	}
	if len(profiles) == 0 {
		err = fmt.Errorf("no media profiles available")
		return
	}
	if token == "" {
		return profiles[0].Token, profiles[0].Name, nil
	}
	for _, p := range profiles {
		if p.Token == token {
			return p.Token, p.Name, nil
		}
	}
	return token, "", nil
}

func streamTransportProtocol(transport string) onvifxsd.TransportProtocol {
	switch strings.ToLower(transport) {
	case "tcp":
		return "TCP"
	case "http":
		return "HTTP"
	case "udp":
		return "UDP"
	default:
		return "RTSP"
	}
}
