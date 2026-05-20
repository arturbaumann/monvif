package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkHostnameCmd = &cobra.Command{
	Use:   "hostname",
	Short: "Show camera hostname",
	RunE:  runNetworkHostname,
}

var (
	networkHostnameFlags  cameraFlags
	networkHostnameFormat string
)

func init() {
	networkHostnameCmd.Flags().StringVar(&networkHostnameFlags.ip, "ip", "", "camera IP address (required)")
	networkHostnameCmd.Flags().IntVar(&networkHostnameFlags.port, "port", 80, "ONVIF port")
	networkHostnameCmd.Flags().StringVar(&networkHostnameFlags.user, "user", "", "username (required)")
	networkHostnameCmd.Flags().StringVar(&networkHostnameFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkHostnameCmd.MarkFlagRequired("ip")
	networkHostnameCmd.MarkFlagRequired("user")
	addFormatFlag(networkHostnameCmd, &networkHostnameFormat, formatTable)
}

func runNetworkHostname(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkHostnameFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkHostnameFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkHostnameFlags.ip, networkHostnameFlags.port, networkHostnameFlags.user, pw)
	if err != nil {
		return err
	}

	h, err := client.GetHostnameInfo(context.Background())
	if err != nil {
		return err
	}

	if networkHostnameFormat == formatJSON {
		return writeJSON(h)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "hostname\t%s\n", h.Name)
	fmt.Fprintf(w, "from_dhcp\t%s\n", boolWord(h.FromDHCP))
	w.Flush()
	return nil
}
