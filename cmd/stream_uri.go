package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
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
	streamTransport    string
	streamFormat       string
)

func init() {
	streamURICmd.Flags().StringVar(&streamFlags.ip, "ip", "", "camera IP address (required)")
	streamURICmd.Flags().IntVar(&streamFlags.port, "port", 80, "ONVIF port")
	streamURICmd.Flags().StringVar(&streamFlags.user, "user", "", "username (required)")
	streamURICmd.Flags().StringVar(&streamFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	streamURICmd.Flags().StringVar(&streamProfileToken, "profile-token", "", "profile token (default: first profile)")
	streamURICmd.Flags().StringVar(&streamTransport, "transport", "rtsp", "transport protocol: rtsp, tcp, http, or udp")
	streamURICmd.MarkFlagRequired("ip")
	streamURICmd.MarkFlagRequired("user")
	addFormatFlag(streamURICmd, &streamFormat, formatTable)
}

func validateTransport(t string) error {
	switch t {
	case "rtsp", "tcp", "http", "udp":
		return nil
	default:
		return fmt.Errorf("invalid --transport %q: must be rtsp, tcp, http, or udp", t)
	}
}

func runStreamURI(cmd *cobra.Command, args []string) error {
	if err := validateFormat(streamFormat); err != nil {
		return err
	}
	if err := validateTransport(streamTransport); err != nil {
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

	token, name, uri, err := client.GetStreamURI(context.Background(), streamProfileToken, camera.StreamURIOptions{
		Transport: streamTransport,
	})
	if err != nil {
		return err
	}

	uri = redactURICredentials(uri)

	if streamFormat == formatJSON {
		return writeJSON(struct {
			ProfileToken string `json:"profile_token"`
			ProfileName  string `json:"profile_name,omitempty"`
			URI          string `json:"uri"`
		}{ProfileToken: token, ProfileName: name, URI: uri})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE_TOKEN\tPROFILE_NAME\tURI")
	fmt.Fprintf(w, "%s\t%s\t%s\n", token, name, uri)
	w.Flush()
	return nil
}
