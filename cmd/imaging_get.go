package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var imagingGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Query current imaging settings for a camera",
	RunE:  runImagingGet,
}

var (
	imagingGetFlags  cameraFlags
	imagingGetFormat string
)

func init() {
	imagingGetCmd.Flags().StringVar(&imagingGetFlags.ip, "ip", "", "camera IP address (required)")
	imagingGetCmd.Flags().IntVar(&imagingGetFlags.port, "port", 80, "ONVIF port")
	imagingGetCmd.Flags().StringVar(&imagingGetFlags.user, "user", "", "username (required)")
	imagingGetCmd.Flags().StringVar(&imagingGetFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	imagingGetCmd.MarkFlagRequired("ip")
	imagingGetCmd.MarkFlagRequired("user")
	addFormatFlag(imagingGetCmd, &imagingGetFormat, formatTable)
}

func runImagingGet(cmd *cobra.Command, args []string) error {
	if err := validateFormat(imagingGetFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(imagingGetFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(imagingGetFlags.ip, imagingGetFlags.port, imagingGetFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	token, err := client.GetVideoSourceToken(ctx)
	if err != nil {
		return err
	}

	settings, err := client.GetImagingSettings(ctx, token)
	if err != nil {
		return err
	}

	if imagingGetFormat == formatJSON {
		return writeJSON(settings)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "brightness\t%.4g\n", settings.Brightness)
	fmt.Fprintf(w, "contrast\t%.4g\n", settings.Contrast)
	fmt.Fprintf(w, "saturation\t%.4g\n", settings.Saturation)
	fmt.Fprintf(w, "sharpness\t%.4g\n", settings.Sharpness)
	if settings.BacklightMode != "" {
		fmt.Fprintf(w, "backlight_mode\t%s\n", settings.BacklightMode)
	}
	if settings.ExposureMode != "" {
		fmt.Fprintf(w, "exposure_mode\t%s\n", settings.ExposureMode)
	}
	if settings.WhiteBalanceMode != "" {
		fmt.Fprintf(w, "white_balance_mode\t%s\n", settings.WhiteBalanceMode)
	}
	if settings.IRCutFilter != "" {
		fmt.Fprintf(w, "ir_cut_filter\t%s\n", settings.IRCutFilter)
	}
	if settings.WDRMode != "" {
		fmt.Fprintf(w, "wdr_mode\t%s\n", settings.WDRMode)
	}
	w.Flush()
	return nil
}
