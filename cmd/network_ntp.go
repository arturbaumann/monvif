package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkNTPCmd = &cobra.Command{
	Use:   "ntp",
	Short: "Show camera NTP configuration",
	RunE:  runNetworkNTP,
}

var (
	networkNTPFlags  cameraFlags
	networkNTPFormat string
)

func init() {
	networkNTPCmd.Flags().StringVar(&networkNTPFlags.ip, "ip", "", "camera IP address (required)")
	networkNTPCmd.Flags().IntVar(&networkNTPFlags.port, "port", 80, "ONVIF port")
	networkNTPCmd.Flags().StringVar(&networkNTPFlags.user, "user", "", "username (required)")
	networkNTPCmd.Flags().StringVar(&networkNTPFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkNTPCmd.MarkFlagRequired("ip")
	networkNTPCmd.MarkFlagRequired("user")
	addFormatFlag(networkNTPCmd, &networkNTPFormat, formatTable)
}

func runNetworkNTP(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkNTPFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkNTPFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkNTPFlags.ip, networkNTPFlags.port, networkNTPFlags.user, pw)
	if err != nil {
		return err
	}

	ntp, err := client.GetNTPInfo(context.Background())
	if err != nil {
		return err
	}

	if networkNTPFormat == formatJSON {
		return writeJSON(ntp)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "from_dhcp\t%s\n", boolWord(ntp.FromDHCP))
	if len(ntp.Manual) > 0 {
		fmt.Fprintf(w, "ntp_servers\t%s\n", strings.Join(ntp.Manual, ", "))
	}
	if len(ntp.FromDHCPSrv) > 0 {
		fmt.Fprintf(w, "ntp_from_dhcp\t%s\n", strings.Join(ntp.FromDHCPSrv, ", "))
	}
	w.Flush()
	return nil
}
