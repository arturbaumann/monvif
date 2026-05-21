package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var streamProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List media profiles with stream URI and optional ffprobe analysis",
	Long: `List RTSP stream profiles for an ONVIF camera.

Without --probe, outputs each profile token, name, and redacted stream URI.

With --probe, runs ffprobe on each stream URI and adds CODEC, RESOLUTION, FPS,
BITRATE, and WORKS columns. The ERROR column is included only when at least one
probe fails, so a clean result stays compact.

Stream URIs are always redacted (credentials replaced with ***).`,
	RunE: runStreamProfiles,
}

var (
	streamProfilesFlags        cameraFlags
	streamProfilesFormat       string
	streamProfilesProbe        bool
	streamProfilesTransport    string
	streamProfilesProbeTimeout time.Duration
)

func init() {
	streamProfilesCmd.Flags().StringVar(&streamProfilesFlags.ip, "ip", "", "camera IP address (required)")
	streamProfilesCmd.Flags().IntVar(&streamProfilesFlags.port, "port", 80, "ONVIF port")
	streamProfilesCmd.Flags().StringVar(&streamProfilesFlags.user, "user", "", "username (required)")
	streamProfilesCmd.Flags().StringVar(&streamProfilesFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	streamProfilesCmd.Flags().BoolVar(&streamProfilesProbe, "probe", false,
		"run ffprobe on each stream URI to report codec, resolution, fps, and bitrate")
	streamProfilesCmd.Flags().StringVar(&streamProfilesTransport, "transport", "tcp",
		"RTSP transport for --probe: rtsp, tcp, http, udp (tcp recommended for routed/WAN networks)")
	streamProfilesCmd.Flags().DurationVar(&streamProfilesProbeTimeout, "probe-timeout", 10*time.Second,
		"per-stream ffprobe timeout")
	streamProfilesCmd.MarkFlagRequired("ip")
	streamProfilesCmd.MarkFlagRequired("user")
	addFormatFlag(streamProfilesCmd, &streamProfilesFormat, formatTable)
}

// streamProfileRow is the internal representation used while building results.
type streamProfileRow struct {
	Token     string
	Name      string
	StreamURI string // always redacted
	Probe     *camera.StreamProbeResult
}

// streamProfileJSON is the flat structure emitted for --format json.
// Bitrate and Error are always present so consumers can rely on a stable schema.
type streamProfileJSON struct {
	Token      string  `json:"token"`
	Name       string  `json:"name"`
	StreamURI  string  `json:"stream_uri"`
	Codec      string  `json:"codec,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	Resolution string  `json:"resolution,omitempty"`
	FPS        float64 `json:"fps,omitempty"`
	Bitrate    string  `json:"bitrate"`         // always present; "-" when not reported
	Works      *bool   `json:"works,omitempty"` // nil when --probe not used
	Error      string  `json:"error"`           // always present; "" when no error
}

func rowToJSON(r streamProfileRow) streamProfileJSON {
	j := streamProfileJSON{
		Token:     r.Token,
		Name:      r.Name,
		StreamURI: r.StreamURI,
		Bitrate:   "-",
	}
	if r.Probe == nil {
		return j
	}
	works := r.Probe.Error == ""
	j.Works = &works
	if !works {
		j.Error = r.Probe.Error
		return j
	}
	j.Codec = r.Probe.Codec
	j.Width = r.Probe.Width
	j.Height = r.Probe.Height
	if r.Probe.Width > 0 && r.Probe.Height > 0 {
		j.Resolution = fmt.Sprintf("%dx%d", r.Probe.Width, r.Probe.Height)
	}
	j.FPS = r.Probe.FPS
	if r.Probe.BitrateBps > 0 {
		j.Bitrate = formatBitrate(r.Probe.BitrateBps)
	}
	return j
}

func runStreamProfiles(cmd *cobra.Command, args []string) error {
	if err := validateFormat(streamProfilesFormat); err != nil {
		return err
	}
	if err := validateTransport(streamProfilesTransport); err != nil {
		return err
	}

	pw, err := resolvePassword(streamProfilesFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(streamProfilesFlags.ip, streamProfilesFlags.port, streamProfilesFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	profiles, err := client.GetProfiles(ctx)
	if err != nil {
		return err
	}

	rows := make([]streamProfileRow, 0, len(profiles))
	for _, p := range profiles {
		rawURI, uriErr := client.GetStreamURIForToken(ctx, p.Token, camera.StreamURIOptions{
			Transport: streamProfilesTransport,
		})
		if uriErr != nil {
			fmt.Fprintf(os.Stderr, "warning: profile %s: %v\n", p.Token, uriErr)
			rows = append(rows, streamProfileRow{Token: p.Token, Name: p.Name})
			continue
		}

		row := streamProfileRow{
			Token:     p.Token,
			Name:      p.Name,
			StreamURI: redactURICredentials(rawURI),
		}
		if streamProfilesProbe {
			// Inject ONVIF credentials into the RTSP URI before handing it to
			// ffprobe. The camera URI typically has no userinfo; RTSP auth
			// requires credentials embedded (or passed via -rtsp_transport, but
			// the URI method is universally supported).
			probeURI, injectErr := camera.InjectRTSPCredentials(rawURI, streamProfilesFlags.user, pw)
			if injectErr != nil {
				fmt.Fprintf(os.Stderr, "warning: profile %s: credential injection failed: %v\n", p.Token, injectErr)
				probeURI = rawURI
			}
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[stream profiles] probing profile %s uri=%s\n",
					p.Token, redactPassword(probeURI))
			}
			probeCtx, cancel := context.WithTimeout(ctx, streamProfilesProbeTimeout)
			result := camera.ProbeStream(probeCtx, probeURI, streamProfilesTransport)
			cancel()
			row.Probe = &result
		}
		rows = append(rows, row)
	}

	if streamProfilesFormat == formatJSON {
		out := make([]streamProfileJSON, len(rows))
		for i, r := range rows {
			out[i] = rowToJSON(r)
		}
		return writeJSON(out)
	}

	return renderStreamProfilesTable(os.Stdout, rows, streamProfilesProbe)
}

func renderStreamProfilesTable(out io.Writer, rows []streamProfileRow, probeMode bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	if !probeMode {
		fmt.Fprintln(w, "TOKEN\tNAME\tURI")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Token, r.Name, r.StreamURI)
		}
		w.Flush()
		return nil
	}

	// Check whether any probe result carries an error so we know if ERROR column is needed.
	hasErrors := false
	for _, r := range rows {
		if r.Probe != nil && r.Probe.Error != "" {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		fmt.Fprintln(w, "TOKEN\tNAME\tCODEC\tRESOLUTION\tFPS\tBITRATE\tWORKS\tERROR")
	} else {
		fmt.Fprintln(w, "TOKEN\tNAME\tCODEC\tRESOLUTION\tFPS\tBITRATE\tWORKS")
	}

	for _, r := range rows {
		codec, res, fps, bitrate, works, errMsg := "-", "-", "-", "-", "-", ""
		if r.Probe != nil {
			if r.Probe.Error != "" {
				works = "no"
				errMsg = r.Probe.Error
			} else {
				works = "yes"
				if r.Probe.Codec != "" {
					codec = r.Probe.Codec
				}
				if r.Probe.Width > 0 && r.Probe.Height > 0 {
					res = fmt.Sprintf("%dx%d", r.Probe.Width, r.Probe.Height)
				}
				if r.Probe.FPS > 0 {
					fps = formatFPS(r.Probe.FPS)
				}
				if r.Probe.BitrateBps > 0 {
					bitrate = formatBitrate(r.Probe.BitrateBps)
				}
			}
		}
		if hasErrors {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Token, r.Name, codec, res, fps, bitrate, works, errMsg)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Token, r.Name, codec, res, fps, bitrate, works)
		}
	}
	w.Flush()
	return nil
}

func formatFPS(fps float64) string {
	if fps == math.Trunc(fps) {
		return fmt.Sprintf("%.0f", fps)
	}
	return strconv.FormatFloat(fps, 'f', 2, 64)
}

// formatBitrate formats bits-per-second as a compact human-readable string:
// ≥1 Mbps → "4.2M", ≥1 kbps → "512k", otherwise "500b".
func formatBitrate(bps int64) string {
	switch {
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(bps)/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.0fk", float64(bps)/1_000)
	default:
		return fmt.Sprintf("%db", bps)
	}
}
