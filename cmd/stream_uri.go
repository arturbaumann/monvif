package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var streamURICmd = &cobra.Command{
	Use:   "stream-uri",
	Short: "Get RTSP stream URI for a media profile",
	RunE:  runStreamURI,
}

var (
	streamFlags        cameraFlags
	streamProfileToken string
	streamFormat       string
)

func init() {
	streamURICmd.Flags().StringVar(&streamFlags.ip, "ip", "", "camera IP address (required)")
	streamURICmd.Flags().IntVar(&streamFlags.port, "port", 80, "ONVIF port")
	streamURICmd.Flags().StringVar(&streamFlags.user, "user", "", "username (required)")
	streamURICmd.Flags().StringVar(&streamFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	streamURICmd.Flags().StringVar(&streamProfileToken, "profile-token", "", "profile token (default: first profile)")
	streamURICmd.MarkFlagRequired("ip")
	streamURICmd.MarkFlagRequired("user")
	addFormatFlag(streamURICmd, &streamFormat, formatTable)
}

func runStreamURI(cmd *cobra.Command, args []string) error {
	if err := validateFormat(streamFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(streamFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(streamFlags.ip, streamFlags.port, streamFlags.user, pw)
	if err != nil {
		return err
	}

	token, uri, err := client.GetStreamURI(context.Background(), streamProfileToken)
	if err != nil {
		return err
	}

	if streamFormat == formatJSON {
		return writeJSON(struct {
			Profile string `json:"profile"`
			URI     string `json:"uri"`
		}{Profile: token, URI: uri})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tURI")
	fmt.Fprintf(w, "%s\t%s\n", token, uri)
	w.Flush()
	return nil
}
