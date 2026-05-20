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

var networkGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show a summary of all camera network settings",
	RunE:  runNetworkGet,
}

var (
	networkGetFlags  cameraFlags
	networkGetFormat string
)

func init() {
	networkGetCmd.Flags().StringVar(&networkGetFlags.ip, "ip", "", "camera IP address (required)")
	networkGetCmd.Flags().IntVar(&networkGetFlags.port, "port", 80, "ONVIF port")
	networkGetCmd.Flags().StringVar(&networkGetFlags.user, "user", "", "username (required)")
	networkGetCmd.Flags().StringVar(&networkGetFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkGetCmd.MarkFlagRequired("ip")
	networkGetCmd.MarkFlagRequired("user")
	addFormatFlag(networkGetCmd, &networkGetFormat, formatTable)
}

func runNetworkGet(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkGetFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkGetFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkGetFlags.ip, networkGetFlags.port, networkGetFlags.user, pw)
	if err != nil {
		return err
	}

	s := client.GetNetworkSummary(context.Background())

	if networkGetFormat == formatJSON {
		return writeJSON(s)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")

	if s.Hostname != nil {
		fmt.Fprintf(w, "hostname\t%s\n", s.Hostname.Name)
		fmt.Fprintf(w, "hostname.from_dhcp\t%s\n", boolWord(s.Hostname.FromDHCP))
	}

	for i, iface := range s.Interfaces {
		prefix := "if"
		if len(s.Interfaces) > 1 {
			prefix = fmt.Sprintf("if[%d]", i)
		}
		fmt.Fprintf(w, "%s.token\t%s\n", prefix, iface.Token)
		fmt.Fprintf(w, "%s.enabled\t%s\n", prefix, boolWord(iface.Enabled))
		if iface.Name != "" {
			fmt.Fprintf(w, "%s.name\t%s\n", prefix, iface.Name)
		}
		if iface.HwAddress != "" {
			fmt.Fprintf(w, "%s.mac\t%s\n", prefix, iface.HwAddress)
		}
		if iface.MTU > 0 {
			fmt.Fprintf(w, "%s.mtu\t%d\n", prefix, iface.MTU)
		}
		fmt.Fprintf(w, "%s.ipv4.enabled\t%s\n", prefix, boolWord(iface.IPv4Enabled))
		fmt.Fprintf(w, "%s.ipv4.dhcp\t%s\n", prefix, boolWord(iface.IPv4DHCP))
		for _, m := range iface.IPv4Manual {
			fmt.Fprintf(w, "%s.ipv4.address\t%s\n", prefix, m.String())
		}
		if iface.IPv4FromDHCP != nil {
			fmt.Fprintf(w, "%s.ipv4.from_dhcp\t%s\n", prefix, iface.IPv4FromDHCP.String())
		}
		fmt.Fprintf(w, "%s.ipv6.enabled\t%s\n", prefix, boolWord(iface.IPv6Enabled))
	}

	if len(s.Gateway) > 0 {
		fmt.Fprintf(w, "gateway\t%s\n", strings.Join(s.Gateway, ", "))
	}

	if s.DNS != nil {
		fmt.Fprintf(w, "dns.from_dhcp\t%s\n", boolWord(s.DNS.FromDHCP))
		if s.DNS.SearchDomain != "" {
			fmt.Fprintf(w, "dns.search_domain\t%s\n", s.DNS.SearchDomain)
		}
		if len(s.DNS.ManualAddrs) > 0 {
			fmt.Fprintf(w, "dns.servers\t%s\n", strings.Join(s.DNS.ManualAddrs, ", "))
		}
		if len(s.DNS.FromDHCPAddrs) > 0 {
			fmt.Fprintf(w, "dns.from_dhcp_servers\t%s\n", strings.Join(s.DNS.FromDHCPAddrs, ", "))
		}
	}

	if s.NTP != nil {
		fmt.Fprintf(w, "ntp.from_dhcp\t%s\n", boolWord(s.NTP.FromDHCP))
		if len(s.NTP.Manual) > 0 {
			fmt.Fprintf(w, "ntp.servers\t%s\n", strings.Join(s.NTP.Manual, ", "))
		}
		if len(s.NTP.FromDHCPSrv) > 0 {
			fmt.Fprintf(w, "ntp.from_dhcp_servers\t%s\n", strings.Join(s.NTP.FromDHCPSrv, ", "))
		}
	}

	if len(s.Protocols) == 0 {
		fmt.Fprintf(w, "protocols\tnone reported\n")
	} else {
		for _, p := range s.Protocols {
			ports := portList(p.Ports)
			if ports != "" {
				fmt.Fprintf(w, "proto.%s\t%s port=%s\n", p.Name, boolWord(p.Enabled), ports)
			} else {
				fmt.Fprintf(w, "proto.%s\t%s\n", p.Name, boolWord(p.Enabled))
			}
		}
	}

	w.Flush()

	for _, warn := range s.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warn)
	}
	return nil
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
