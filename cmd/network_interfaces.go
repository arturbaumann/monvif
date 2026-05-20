package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkInterfacesCmd = &cobra.Command{
	Use:   "interfaces",
	Short: "List camera network interfaces",
	RunE:  runNetworkInterfaces,
}

var (
	networkIfaceFlags  cameraFlags
	networkIfaceFormat string
)

func init() {
	networkInterfacesCmd.Flags().StringVar(&networkIfaceFlags.ip, "ip", "", "camera IP address (required)")
	networkInterfacesCmd.Flags().IntVar(&networkIfaceFlags.port, "port", 80, "ONVIF port")
	networkInterfacesCmd.Flags().StringVar(&networkIfaceFlags.user, "user", "", "username (required)")
	networkInterfacesCmd.Flags().StringVar(&networkIfaceFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkInterfacesCmd.MarkFlagRequired("ip")
	networkInterfacesCmd.MarkFlagRequired("user")
	addFormatFlag(networkInterfacesCmd, &networkIfaceFormat, formatTable)
}

func runNetworkInterfaces(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkIfaceFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkIfaceFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkIfaceFlags.ip, networkIfaceFlags.port, networkIfaceFlags.user, pw)
	if err != nil {
		return err
	}

	ifaces, err := client.GetNetworkInterfaces(context.Background())
	if err != nil {
		return err
	}

	if networkIfaceFormat == formatJSON {
		return writeJSON(ifaces)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOKEN\tENABLED\tNAME\tMAC\tMTU\tIPv4_DHCP\tIPv4_ADDRESS\tIPv6")
	for _, iface := range ifaces {
		addr := ""
		for _, m := range iface.IPv4Manual {
			addr = m.String()
		}
		if addr == "" && iface.IPv4FromDHCP != nil {
			addr = iface.IPv4FromDHCP.String()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			iface.Token,
			boolMark(iface.Enabled),
			iface.Name,
			iface.HwAddress,
			iface.MTU,
			boolMark(iface.IPv4DHCP),
			addr,
			boolMark(iface.IPv6Enabled),
		)
	}
	w.Flush()
	return nil
}
