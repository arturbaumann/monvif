package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var imagingSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update imaging settings for a camera",
	RunE:  runImagingSet,
}

var (
	imagingSetFlags      cameraFlags
	imagingSetFormat     string
	imagingSetBrightness float64
	imagingSetContrast   float64
	imagingSetSaturation float64
	imagingSetSharpness  float64
	imagingSetYes        bool
	imagingSetDryRun     bool
)

func init() {
	imagingSetCmd.Flags().StringVar(&imagingSetFlags.ip, "ip", "", "camera IP address (required)")
	imagingSetCmd.Flags().IntVar(&imagingSetFlags.port, "port", 80, "ONVIF port")
	imagingSetCmd.Flags().StringVar(&imagingSetFlags.user, "user", "", "username (required)")
	imagingSetCmd.Flags().StringVar(&imagingSetFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	imagingSetCmd.Flags().Float64Var(&imagingSetBrightness, "brightness", 0, "brightness value")
	imagingSetCmd.Flags().Float64Var(&imagingSetContrast, "contrast", 0, "contrast value")
	imagingSetCmd.Flags().Float64Var(&imagingSetSaturation, "saturation", 0, "saturation value")
	imagingSetCmd.Flags().Float64Var(&imagingSetSharpness, "sharpness", 0, "sharpness value")
	imagingSetCmd.Flags().BoolVar(&imagingSetYes, "yes", false, "apply changes to the camera")
	imagingSetCmd.Flags().BoolVar(&imagingSetDryRun, "dry-run", false, "preview changes without applying")
	imagingSetCmd.MarkFlagRequired("ip")
	imagingSetCmd.MarkFlagRequired("user")
	addFormatFlag(imagingSetCmd, &imagingSetFormat, formatTable)
}

func buildImagingUpdate(cmd *cobra.Command) camera.ImagingUpdate {
	var u camera.ImagingUpdate
	if cmd.Flags().Changed("brightness") {
		v := imagingSetBrightness
		u.Brightness = &v
	}
	if cmd.Flags().Changed("contrast") {
		v := imagingSetContrast
		u.Contrast = &v
	}
	if cmd.Flags().Changed("saturation") {
		v := imagingSetSaturation
		u.Saturation = &v
	}
	if cmd.Flags().Changed("sharpness") {
		v := imagingSetSharpness
		u.Sharpness = &v
	}
	return u
}

func validateImagingSetArgs(u camera.ImagingUpdate, yes, dryRun bool) error {
	if u.IsEmpty() {
		return fmt.Errorf("no imaging fields specified; use --brightness, --contrast, --saturation, or --sharpness")
	}
	if !dryRun && !yes {
		return fmt.Errorf("imaging set modifies camera settings; pass --yes to apply or --dry-run to preview")
	}
	return nil
}

func runImagingSet(cmd *cobra.Command, args []string) error {
	if err := validateFormat(imagingSetFormat); err != nil {
		return err
	}

	u := buildImagingUpdate(cmd)
	if err := validateImagingSetArgs(u, imagingSetYes, imagingSetDryRun); err != nil {
		return err
	}

	pw, err := resolvePassword(imagingSetFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(imagingSetFlags.ip, imagingSetFlags.port, imagingSetFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	token, err := client.GetVideoSourceToken(ctx)
	if err != nil {
		return err
	}

	var before, after camera.ImagingSettings
	if imagingSetDryRun {
		before, after, err = client.PreviewImagingUpdate(ctx, token, u)
	} else {
		before, after, err = client.UpdateImagingSettings(ctx, token, u)
	}
	if err != nil {
		return err
	}

	if imagingSetFormat == formatJSON {
		return writeJSON(struct {
			DryRun bool                   `json:"dry_run"`
			Before camera.ImagingSettings `json:"before"`
			After  camera.ImagingSettings `json:"after"`
		}{DryRun: imagingSetDryRun, Before: before, After: after})
	}

	label := "AFTER (applied)"
	if imagingSetDryRun {
		label = "AFTER (preview)"
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "FIELD\tBEFORE\t%s\n", label)
	if u.Brightness != nil {
		fmt.Fprintf(w, "brightness\t%.4g\t%.4g\n", before.Brightness, after.Brightness)
	}
	if u.Contrast != nil {
		fmt.Fprintf(w, "contrast\t%.4g\t%.4g\n", before.Contrast, after.Contrast)
	}
	if u.Saturation != nil {
		fmt.Fprintf(w, "saturation\t%.4g\t%.4g\n", before.Saturation, after.Saturation)
	}
	if u.Sharpness != nil {
		fmt.Fprintf(w, "sharpness\t%.4g\t%.4g\n", before.Sharpness, after.Sharpness)
	}
	w.Flush()
	return nil
}
