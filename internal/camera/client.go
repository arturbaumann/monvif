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

// GetStreamURI returns the RTSP stream URI for the given profile token.
// If token is empty, the first available profile is used.
func (c *Client) GetStreamURI(ctx context.Context, token string) (string, string, error) {
	if token == "" {
		profiles, err := c.GetProfiles(ctx)
		if err != nil {
			return "", "", err
		}
		if len(profiles) == 0 {
			return "", "", fmt.Errorf("no media profiles available")
		}
		token = profiles[0].Token
	}

	resp, err := sdkmedia.Call_GetStreamUri(ctx, c.dev, media.GetStreamUri{
		ProfileToken: onvifxsd.ReferenceToken(token),
		StreamSetup: onvifxsd.StreamSetup{
			Stream:    onvifxsd.StreamType("RTP-Unicast"),
			Transport: onvifxsd.Transport{Protocol: onvifxsd.TransportProtocol("RTSP")},
		},
	})
	if err != nil {
		return token, "", fmt.Errorf("GetStreamUri: %w", err)
	}
	uri := strings.TrimSpace(string(resp.MediaUri.Uri))
	return token, uri, nil
}
