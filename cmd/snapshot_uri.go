package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var snapshotURICmd = &cobra.Command{
	Use:   "snapshot-uri",
	Short: "Get snapshot URI for a media profile",
	RunE:  runSnapshotURI,
}

var (
	snapshotFlags        cameraFlags
	snapshotProfileToken string
	snapshotFormat       string
)

func init() {
	snapshotURICmd.Flags().StringVar(&snapshotFlags.ip, "ip", "", "camera IP address (required)")
	snapshotURICmd.Flags().IntVar(&snapshotFlags.port, "port", 80, "ONVIF port")
	snapshotURICmd.Flags().StringVar(&snapshotFlags.user, "user", "", "username (required)")
	snapshotURICmd.Flags().StringVar(&snapshotFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	snapshotURICmd.Flags().StringVar(&snapshotProfileToken, "profile-token", "", "profile token (default: first profile)")
	snapshotURICmd.MarkFlagRequired("ip")
	snapshotURICmd.MarkFlagRequired("user")
	addFormatFlag(snapshotURICmd, &snapshotFormat, formatTable)
}

func runSnapshotURI(cmd *cobra.Command, args []string) error {
	if err := validateFormat(snapshotFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(snapshotFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(snapshotFlags.ip, snapshotFlags.port, snapshotFlags.user, pw)
	if err != nil {
		return err
	}

	token, name, uri, err := client.GetSnapshotURI(context.Background(), snapshotProfileToken)
	if err != nil {
		return err
	}

	uri = redactURICredentials(uri)

	if snapshotFormat == formatJSON {
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
