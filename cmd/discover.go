package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/arturbaumann/monvif/internal/discovery"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover ONVIF cameras on the local LAN via WS-Discovery",
	Long: `Sends a WS-Discovery multicast probe (UDP port 3702) and prints cameras that respond.

Note: discovery only finds cameras that actively respond to multicast probes.
Some cameras support ONVIF queries but have discovery disabled or are on a network
that blocks multicast. If a camera does not appear here, try querying it directly
with its IP address using 'monvif info' or 'monvif check'.

Typical workflow:
  1. Run 'monvif discover' to find cameras that announce themselves.
  2. For cameras not found by discovery, scan with nmap or maintain a cameras.tsv
     inventory and run 'monvif check --file cameras.tsv --user <user>'.
  3. Query individual cameras with 'monvif info', 'monvif capabilities',
     'monvif profiles', or 'monvif stream-uri'.`,
	RunE: runDiscover,
}

var (
	discoverTimeout   time.Duration
	discoverInterface string
)

func init() {
	discoverCmd.Flags().DurationVar(&discoverTimeout, "timeout", 5*time.Second, "discovery timeout")
	discoverCmd.Flags().StringVar(&discoverInterface, "interface", "", "network interface to use (default: auto-detect)")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "Probing for ONVIF devices (timeout %s)...\n", discoverTimeout)

	devices, err := discovery.Discover(discoverInterface, discoverTimeout)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No ONVIF devices found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tXADDRS\tTYPES\tSCOPES")
	for _, d := range devices {
		name := d.Name
		if name == "" {
			name = "(unknown)"
		}
		xaddrs := strings.Join(d.XAddrs, " ")
		types := d.Types
		// Show only first 3 scopes to keep the table readable.
		scopes := d.Scopes
		if len(scopes) > 3 {
			scopes = append(scopes[:3], "...")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, xaddrs, types, strings.Join(scopes, " "))
	}
	w.Flush()
	return nil
}
