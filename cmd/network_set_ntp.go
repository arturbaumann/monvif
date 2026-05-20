package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkSetNTPCmd = &cobra.Command{
	Use:   "set-ntp",
	Short: "Set camera NTP configuration",
	Long: `Set camera NTP configuration via ONVIF SetNTP.

WARNING: Changing network settings may make the camera unreachable.
Use --dry-run to preview the change before applying.

Note: due to a library limitation, only the first --server value is sent
to the camera when using manual NTP mode.`,
	RunE: runNetworkSetNTP,
}

var (
	networkSetNTPFlags   cameraFlags
	networkSetNTPDHCP    bool
	networkSetNTPServers []string
	networkSetNTPDryRun  bool
	networkSetNTPYes     bool
	networkSetNTPFormat  string
)

// ntpServerRe accepts IPv4 addresses and DNS hostnames (including pool.ntp.org style).
var ntpServerRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)

func init() {
	networkSetNTPCmd.Flags().StringVar(&networkSetNTPFlags.ip, "ip", "", "camera IP address (required)")
	networkSetNTPCmd.Flags().IntVar(&networkSetNTPFlags.port, "port", 80, "ONVIF port")
	networkSetNTPCmd.Flags().StringVar(&networkSetNTPFlags.user, "user", "", "username (required)")
	networkSetNTPCmd.Flags().StringVar(&networkSetNTPFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkSetNTPCmd.Flags().BoolVar(&networkSetNTPDHCP, "dhcp", false, "use NTP from DHCP (mutually exclusive with --server)")
	networkSetNTPCmd.Flags().StringArrayVar(&networkSetNTPServers, "server", nil, "NTP server IP or hostname (repeatable)")
	networkSetNTPCmd.Flags().BoolVar(&networkSetNTPDryRun, "dry-run", false, "preview change without applying")
	networkSetNTPCmd.Flags().BoolVar(&networkSetNTPYes, "yes", false, "apply changes to the camera")
	networkSetNTPCmd.MarkFlagRequired("ip")
	networkSetNTPCmd.MarkFlagRequired("user")
	addFormatFlag(networkSetNTPCmd, &networkSetNTPFormat, formatTable)
}

func validateSetNTPArgs(dhcp bool, servers []string, dryRun, yes bool) error {
	if dhcp && len(servers) > 0 {
		return fmt.Errorf("--dhcp and --server are mutually exclusive")
	}
	if !dhcp && len(servers) == 0 {
		return fmt.Errorf("either --dhcp or at least one --server is required")
	}
	for _, s := range servers {
		if !ntpServerRe.MatchString(s) {
			return fmt.Errorf("invalid --server %q: must be an IP address or hostname", s)
		}
	}
	if dryRun && yes {
		return fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !dryRun && !yes {
		return fmt.Errorf("network set-ntp modifies camera settings; pass --yes to apply or --dry-run to preview")
	}
	return nil
}

func runNetworkSetNTP(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkSetNTPFormat); err != nil {
		return err
	}
	if err := validateSetNTPArgs(networkSetNTPDHCP, networkSetNTPServers, networkSetNTPDryRun, networkSetNTPYes); err != nil {
		return err
	}

	pw, err := resolvePassword(networkSetNTPFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkSetNTPFlags.ip, networkSetNTPFlags.port, networkSetNTPFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	current, _ := client.GetNTPInfo(ctx)

	cfg := camera.SetNTPConfig{
		FromDHCP: networkSetNTPDHCP,
		Servers:  networkSetNTPServers,
	}

	if networkSetNTPDryRun {
		printSetNTPPreview(current, cfg, networkSetNTPFormat)
		return nil
	}

	fmt.Fprintln(os.Stderr, "WARNING: Changing network settings may make the camera unreachable.")

	if err := client.SetNTP(ctx, cfg); err != nil {
		return err
	}

	if networkSetNTPFormat == formatJSON {
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

func printSetNTPPreview(current camera.NTPInfo, cfg camera.SetNTPConfig, format string) {
	currentServers := strings.Join(append(current.Manual, current.FromDHCPSrv...), ", ")
	requestedServers := strings.Join(cfg.Servers, ", ")
	if cfg.FromDHCP {
		requestedServers = "(from DHCP)"
	}

	if format == formatJSON {
		_ = writeJSON(struct {
			DryRun    bool                `json:"dry_run"`
			Current   camera.NTPInfo      `json:"current"`
			Requested camera.SetNTPConfig `json:"requested"`
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
