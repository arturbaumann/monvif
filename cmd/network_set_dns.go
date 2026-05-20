package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkSetDNSCmd = &cobra.Command{
	Use:   "set-dns",
	Short: "Set camera DNS configuration",
	Long: `Set camera DNS configuration via ONVIF SetDNS.

WARNING: Changing network settings may make the camera unreachable.
Use --dry-run to preview the change before applying.

Note: due to a library limitation, only the first --server value is sent
to the camera when using manual DNS mode.`,
	RunE: runNetworkSetDNS,
}

var (
	networkSetDNSFlags   cameraFlags
	networkSetDNSDHCP    bool
	networkSetDNSServers []string
	networkSetDNSDryRun  bool
	networkSetDNSYes     bool
	networkSetDNSFormat  string
)

func init() {
	networkSetDNSCmd.Flags().StringVar(&networkSetDNSFlags.ip, "ip", "", "camera IP address (required)")
	networkSetDNSCmd.Flags().IntVar(&networkSetDNSFlags.port, "port", 80, "ONVIF port")
	networkSetDNSCmd.Flags().StringVar(&networkSetDNSFlags.user, "user", "", "username (required)")
	networkSetDNSCmd.Flags().StringVar(&networkSetDNSFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkSetDNSCmd.Flags().BoolVar(&networkSetDNSDHCP, "dhcp", false, "use DNS from DHCP (mutually exclusive with --server)")
	networkSetDNSCmd.Flags().StringArrayVar(&networkSetDNSServers, "server", nil, "DNS server IP address (repeatable)")
	networkSetDNSCmd.Flags().BoolVar(&networkSetDNSDryRun, "dry-run", false, "preview change without applying")
	networkSetDNSCmd.Flags().BoolVar(&networkSetDNSYes, "yes", false, "apply changes to the camera")
	networkSetDNSCmd.MarkFlagRequired("ip")
	networkSetDNSCmd.MarkFlagRequired("user")
	addFormatFlag(networkSetDNSCmd, &networkSetDNSFormat, formatTable)
}

func validateSetDNSArgs(dhcp bool, servers []string, dryRun, yes bool) error {
	if dhcp && len(servers) > 0 {
		return fmt.Errorf("--dhcp and --server are mutually exclusive")
	}
	if !dhcp && len(servers) == 0 {
		return fmt.Errorf("either --dhcp or at least one --server is required")
	}
	for _, s := range servers {
		if net.ParseIP(s) == nil || !strings.Contains(s, ".") {
			return fmt.Errorf("invalid --server %q: must be an IPv4 address", s)
		}
	}
	if dryRun && yes {
		return fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !dryRun && !yes {
		return fmt.Errorf("network set-dns modifies camera settings; pass --yes to apply or --dry-run to preview")
	}
	return nil
}

func runNetworkSetDNS(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkSetDNSFormat); err != nil {
		return err
	}
	if err := validateSetDNSArgs(networkSetDNSDHCP, networkSetDNSServers, networkSetDNSDryRun, networkSetDNSYes); err != nil {
		return err
	}

	pw, err := resolvePassword(networkSetDNSFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkSetDNSFlags.ip, networkSetDNSFlags.port, networkSetDNSFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	current, _ := client.GetDNSInfo(ctx)

	cfg := camera.SetDNSConfig{
		FromDHCP: networkSetDNSDHCP,
		Servers:  networkSetDNSServers,
	}

	if networkSetDNSDryRun {
		printSetDNSPreview(current, cfg, networkSetDNSFormat)
		return nil
	}

	fmt.Fprintln(os.Stderr, "WARNING: Changing network settings may make the camera unreachable.")

	if err := client.SetDNS(ctx, cfg); err != nil {
		return err
	}

	if networkSetDNSFormat == formatJSON {
		return writeJSON(struct {
			Applied  bool     `json:"applied"`
			FromDHCP bool     `json:"from_dhcp"`
			Servers  []string `json:"servers,omitempty"`
		}{Applied: true, FromDHCP: cfg.FromDHCP, Servers: cfg.Servers})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "applied\tyes\n")
	fmt.Fprintf(w, "from_dhcp\t%s\n", boolWord(cfg.FromDHCP))
	if len(cfg.Servers) > 0 {
		fmt.Fprintf(w, "server_sent\t%s\n", cfg.Servers[0])
		if len(cfg.Servers) > 1 {
			fmt.Fprintf(w, "servers_skipped\t%s (library limitation: only first server sent)\n",
				strings.Join(cfg.Servers[1:], ", "))
		}
	}
	w.Flush()
	return nil
}

func printSetDNSPreview(current camera.DNSInfo, cfg camera.SetDNSConfig, format string) {
	currentServers := strings.Join(append(current.ManualAddrs, current.FromDHCPAddrs...), ", ")
	requestedServers := strings.Join(cfg.Servers, ", ")
	if cfg.FromDHCP {
		requestedServers = "(from DHCP)"
	}

	if format == formatJSON {
		_ = writeJSON(struct {
			DryRun    bool                `json:"dry_run"`
			Current   camera.DNSInfo      `json:"current"`
			Requested camera.SetDNSConfig `json:"requested"`
		}{DryRun: true, Current: current, Requested: cfg})
		return
	}

	fmt.Fprintln(os.Stderr, "WARNING: Changing network settings may make the camera unreachable.")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tCURRENT\tREQUESTED")
	fmt.Fprintf(w, "from_dhcp\t%s\t%s\n", boolWord(current.FromDHCP), boolWord(cfg.FromDHCP))
	fmt.Fprintf(w, "servers\t%s\t%s\n", currentServers, requestedServers)
	w.Flush()
	fmt.Println()
	fmt.Println("Dry run: no changes applied.")
}
