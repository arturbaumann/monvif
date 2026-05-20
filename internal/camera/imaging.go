package camera

import (
	"context"
	"fmt"

	oimaging "github.com/use-go/onvif/Imaging"
	omedia "github.com/use-go/onvif/media"
	"github.com/use-go/onvif/sdk"
	sdkmedia "github.com/use-go/onvif/sdk/media"
	"github.com/use-go/onvif/xsd"
	onvifxsd "github.com/use-go/onvif/xsd/onvif"
)

// ImagingSettings is a simplified view of camera picture settings for display and updates.
type ImagingSettings struct {
	Brightness       float64 `json:"brightness"`
	Contrast         float64 `json:"contrast"`
	Saturation       float64 `json:"saturation"` // ColorSaturation in ONVIF
	Sharpness        float64 `json:"sharpness"`
	BacklightMode    string  `json:"backlight_mode,omitempty"`
	ExposureMode     string  `json:"exposure_mode,omitempty"`
	WhiteBalanceMode string  `json:"white_balance_mode,omitempty"`
	IRCutFilter      string  `json:"ir_cut_filter,omitempty"`
	WDRMode          string  `json:"wdr_mode,omitempty"`
}

// ImagingUpdate holds the picture fields to change; nil means leave unchanged.
type ImagingUpdate struct {
	Brightness *float64
	Contrast   *float64
	Saturation *float64
	Sharpness  *float64
}

// IsEmpty returns true when no fields are requested to change.
func (u ImagingUpdate) IsEmpty() bool {
	return u.Brightness == nil && u.Contrast == nil &&
		u.Saturation == nil && u.Sharpness == nil
}

// GetVideoSourceToken returns the first video source token via the media service.
func (c *Client) GetVideoSourceToken(ctx context.Context) (string, error) {
	resp, err := sdkmedia.Call_GetVideoSources(ctx, c.dev, omedia.GetVideoSources{})
	if err != nil {
		return "", fmt.Errorf("GetVideoSources: %w", err)
	}
	token := string(resp.VideoSources.Token)
	if token == "" {
		return "", fmt.Errorf("no video source token found")
	}
	return token, nil
}

// GetImagingSettings fetches current imaging settings for the given video source token.
func (c *Client) GetImagingSettings(ctx context.Context, videoSourceToken string) (ImagingSettings, error) {
	raw, err := c.getRawImagingSettings(ctx, videoSourceToken)
	if err != nil {
		return ImagingSettings{}, err
	}
	return rawToImagingSettings(raw), nil
}

// PreviewImagingUpdate returns what UpdateImagingSettings would produce without
// calling SetImagingSettings.
func (c *Client) PreviewImagingUpdate(ctx context.Context, videoSourceToken string, u ImagingUpdate) (before, after ImagingSettings, err error) {
	raw, modified, err := c.computeImagingUpdate(ctx, videoSourceToken, u)
	if err != nil {
		return
	}
	before = rawToImagingSettings(raw)
	after = rawToImagingSettings(modified)
	return
}

// UpdateImagingSettings reads current settings, applies non-nil fields in u,
// calls SetImagingSettings, and returns before/after snapshots.
func (c *Client) UpdateImagingSettings(ctx context.Context, videoSourceToken string, u ImagingUpdate) (before, after ImagingSettings, err error) {
	raw, modified, err := c.computeImagingUpdate(ctx, videoSourceToken, u)
	if err != nil {
		return
	}
	before = rawToImagingSettings(raw)
	after = rawToImagingSettings(modified)
	err = c.setRawImagingSettings(ctx, videoSourceToken, modified)
	return
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (c *Client) computeImagingUpdate(ctx context.Context, token string, u ImagingUpdate) (raw, modified onvifxsd.ImagingSettings20, err error) {
	raw, err = c.getRawImagingSettings(ctx, token)
	if err != nil {
		return
	}
	modified = raw
	if u.Brightness != nil {
		modified.Brightness = *u.Brightness
	}
	if u.Contrast != nil {
		modified.Contrast = *u.Contrast
	}
	if u.Saturation != nil {
		modified.ColorSaturation = *u.Saturation
	}
	if u.Sharpness != nil {
		modified.Sharpness = *u.Sharpness
	}
	return
}

func (c *Client) getRawImagingSettings(ctx context.Context, token string) (onvifxsd.ImagingSettings20, error) {
	type settingsResponse struct {
		ImagingSettings onvifxsd.ImagingSettings20
	}
	type envelope struct {
		Header struct{}
		Body   struct {
			GetImagingSettingsResponse settingsResponse
		}
	}
	httpResp, err := c.dev.CallMethod(oimaging.GetImagingSettings{
		VideoSourceToken: onvifxsd.ReferenceToken(token),
	})
	if err != nil {
		return onvifxsd.ImagingSettings20{}, fmt.Errorf("GetImagingSettings: %w", err)
	}
	var reply envelope
	if err := sdk.ReadAndParse(ctx, httpResp, &reply, "GetImagingSettings"); err != nil {
		return onvifxsd.ImagingSettings20{}, err
	}
	return reply.Body.GetImagingSettingsResponse.ImagingSettings, nil
}

func (c *Client) setRawImagingSettings(ctx context.Context, token string, s onvifxsd.ImagingSettings20) error {
	type envelope struct {
		Header struct{}
		Body   struct {
			SetImagingSettingsResponse struct{}
		}
	}
	httpResp, err := c.dev.CallMethod(oimaging.SetImagingSettings{
		VideoSourceToken: onvifxsd.ReferenceToken(token),
		ImagingSettings:  s,
		ForcePersistence: xsd.Boolean(true),
	})
	if err != nil {
		return fmt.Errorf("SetImagingSettings: %w", err)
	}
	var reply envelope
	return sdk.ReadAndParse(ctx, httpResp, &reply, "SetImagingSettings")
}

func rawToImagingSettings(raw onvifxsd.ImagingSettings20) ImagingSettings {
	return ImagingSettings{
		Brightness:       raw.Brightness,
		Contrast:         raw.Contrast,
		Saturation:       raw.ColorSaturation,
		Sharpness:        raw.Sharpness,
		BacklightMode:    string(raw.BacklightCompensation.Mode),
		ExposureMode:     string(raw.Exposure.Mode),
		WhiteBalanceMode: string(raw.WhiteBalance.Mode),
		IRCutFilter:      string(raw.IrCutFilter),
		WDRMode:          string(raw.WideDynamicRange.Mode),
	}
}
