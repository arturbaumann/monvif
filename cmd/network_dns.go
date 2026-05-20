package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkDNSCmd = &cobra.Command{
	Use:   "dns",
	Short: "Show camera DNS configuration",
	RunE:  runNetworkDNS,
}

var (
	networkDNSFlags  cameraFlags
	networkDNSFormat string
)

func init() {
	networkDNSCmd.Flags().StringVar(&networkDNSFlags.ip, "ip", "", "camera IP address (required)")
	networkDNSCmd.Flags().IntVar(&networkDNSFlags.port, "port", 80, "ONVIF port")
	networkDNSCmd.Flags().StringVar(&networkDNSFlags.user, "user", "", "username (required)")
	networkDNSCmd.Flags().StringVar(&networkDNSFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkDNSCmd.MarkFlagRequired("ip")
	networkDNSCmd.MarkFlagRequired("user")
	addFormatFlag(networkDNSCmd, &networkDNSFormat, formatTable)
}

func runNetworkDNS(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkDNSFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkDNSFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkDNSFlags.ip, networkDNSFlags.port, networkDNSFlags.user, pw)
	if err != nil {
		return err
	}

	dns, err := client.GetDNSInfo(context.Background())
	if err != nil {
		return err
	}

	if networkDNSFormat == formatJSON {
		return writeJSON(dns)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "from_dhcp\t%s\n", boolWord(dns.FromDHCP))
	if dns.SearchDomain != "" {
		fmt.Fprintf(w, "search_domain\t%s\n", dns.SearchDomain)
	}
	if len(dns.ManualAddrs) > 0 {
		fmt.Fprintf(w, "dns_servers\t%s\n", strings.Join(dns.ManualAddrs, ", "))
	}
	if len(dns.FromDHCPAddrs) > 0 {
		fmt.Fprintf(w, "dns_from_dhcp\t%s\n", strings.Join(dns.FromDHCPAddrs, ", "))
	}
	w.Flush()
	return nil
}
