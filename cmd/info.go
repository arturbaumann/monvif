package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Query device information from an ONVIF camera",
	RunE:  runInfo,
}

var (
	infoFlags  cameraFlags
	infoFormat string
)

func init() {
	infoCmd.Flags().StringVar(&infoFlags.ip, "ip", "", "camera IP address (required)")
	infoCmd.Flags().IntVar(&infoFlags.port, "port", 80, "ONVIF port")
	infoCmd.Flags().StringVar(&infoFlags.user, "user", "", "username (required)")
	infoCmd.Flags().StringVar(&infoFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	infoCmd.MarkFlagRequired("ip")
	infoCmd.MarkFlagRequired("user")
	addFormatFlag(infoCmd, &infoFormat, formatJSON)
}

func runInfo(cmd *cobra.Command, args []string) error {
	if err := validateFormat(infoFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(infoFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(infoFlags.ip, infoFlags.port, infoFlags.user, pw)
	if err != nil {
		return err
	}

	info, err := client.GetDeviceInfo(context.Background())
	if err != nil {
		return err
	}

	if infoFormat == formatJSON {
		return writeJSON(info)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	rows := []struct{ k, v string }{
		{"Manufacturer", info.Manufacturer},
		{"Model", info.Model},
		{"Firmware", info.FirmwareVersion},
		{"Serial", info.SerialNumber},
		{"HardwareID", info.HardwareID},
	}
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\n", r.k, r.v)
	}
	w.Flush()
	return nil
}
