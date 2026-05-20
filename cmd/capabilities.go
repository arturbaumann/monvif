package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var capabilitiesCmd = &cobra.Command{
	Use:   "capabilities",
	Short: "Query ONVIF service capability URLs",
	RunE:  runCapabilities,
}

var (
	capsFlags  cameraFlags
	capsFormat string
)

func init() {
	capabilitiesCmd.Flags().StringVar(&capsFlags.ip, "ip", "", "camera IP address (required)")
	capabilitiesCmd.Flags().IntVar(&capsFlags.port, "port", 80, "ONVIF port")
	capabilitiesCmd.Flags().StringVar(&capsFlags.user, "user", "", "username (required)")
	capabilitiesCmd.Flags().StringVar(&capsFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	capabilitiesCmd.MarkFlagRequired("ip")
	capabilitiesCmd.MarkFlagRequired("user")
	addFormatFlag(capabilitiesCmd, &capsFormat, formatTable)
}

type capsJSON struct {
	Device  string `json:"device,omitempty"`
	Media   string `json:"media,omitempty"`
	Imaging string `json:"imaging,omitempty"`
	Events  string `json:"events,omitempty"`
	PTZ     string `json:"ptz,omitempty"`
}

func runCapabilities(cmd *cobra.Command, args []string) error {
	if err := validateFormat(capsFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(capsFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(capsFlags.ip, capsFlags.port, capsFlags.user, pw)
	if err != nil {
		return err
	}

	caps, err := client.GetCapabilities(context.Background())
	if err != nil {
		return err
	}

	if capsFormat == formatJSON {
		return writeJSON(capsJSON{
			Device:  caps.DeviceXAddr,
			Media:   caps.MediaXAddr,
			Imaging: caps.ImagingXAddr,
			Events:  caps.EventsXAddr,
			PTZ:     caps.PTZXAddr,
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tXADDR")
	rows := []struct{ svc, addr string }{
		{"Device", caps.DeviceXAddr},
		{"Media", caps.MediaXAddr},
		{"Imaging", caps.ImagingXAddr},
		{"Events", caps.EventsXAddr},
		{"PTZ", caps.PTZXAddr},
	}
	for _, r := range rows {
		if r.addr != "" {
			fmt.Fprintf(w, "%s\t%s\n", r.svc, r.addr)
		}
	}
	w.Flush()
	return nil
}
