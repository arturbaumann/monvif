package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkSetIPCmd = &cobra.Command{
	Use:   "set-ip",
	Short: "Set camera IPv4 configuration",
	Long: `Set camera IPv4 configuration via ONVIF SetNetworkInterfaces.

WARNING: Changing network settings may make the camera unreachable.
Always run 'monvif network interfaces' first to get the interface token.
Use --dry-run to preview the change before applying.`,
	RunE: runNetworkSetIP,
}

var (
	networkSetIPFlags     cameraFlags
	networkSetIPInterface string
	networkSetIPDHCP      bool
	networkSetIPAddress   string
	networkSetIPPrefixLen int
	networkSetIPGateway   string
	networkSetIPDryRun    bool
	networkSetIPYes       bool
	networkSetIPFormat    string
)

func init() {
	networkSetIPCmd.Flags().StringVar(&networkSetIPFlags.ip, "ip", "", "camera IP address (required)")
	networkSetIPCmd.Flags().IntVar(&networkSetIPFlags.port, "port", 80, "ONVIF port")
	networkSetIPCmd.Flags().StringVar(&networkSetIPFlags.user, "user", "", "username (required)")
	networkSetIPCmd.Flags().StringVar(&networkSetIPFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkSetIPCmd.Flags().StringVar(&networkSetIPInterface, "interface", "", "interface token from 'network interfaces' (required)")
	networkSetIPCmd.Flags().BoolVar(&networkSetIPDHCP, "dhcp", false, "enable DHCP (mutually exclusive with --address)")
	networkSetIPCmd.Flags().StringVar(&networkSetIPAddress, "address", "", "static IPv4 address")
	networkSetIPCmd.Flags().IntVar(&networkSetIPPrefixLen, "prefix-length", 0, "IPv4 prefix length (1-32)")
	networkSetIPCmd.Flags().StringVar(&networkSetIPGateway, "gateway", "", "default gateway IPv4 address")
	networkSetIPCmd.Flags().BoolVar(&networkSetIPDryRun, "dry-run", false, "preview change without applying")
	networkSetIPCmd.Flags().BoolVar(&networkSetIPYes, "yes", false, "apply changes to the camera")
	networkSetIPCmd.MarkFlagRequired("ip")
	networkSetIPCmd.MarkFlagRequired("user")
	networkSetIPCmd.MarkFlagRequired("interface")
	addFormatFlag(networkSetIPCmd, &networkSetIPFormat, formatTable)
}

func validateSetIPArgs(iface, address, gateway string, prefixLen int, dhcp, dryRun, yes bool) error {
	if dhcp && address != "" {
		return fmt.Errorf("--dhcp and --address are mutually exclusive")
	}
	if !dhcp && address == "" {
		return fmt.Errorf("either --dhcp or --address (with --prefix-length) is required")
	}
	if !dhcp {
		if net.ParseIP(address) == nil || !strings.Contains(address, ".") {
			return fmt.Errorf("invalid --address %q: must be an IPv4 address", address)
		}
		if prefixLen < 1 || prefixLen > 32 {
			return fmt.Errorf("--prefix-length must be between 1 and 32, got %d", prefixLen)
		}
		if gateway != "" {
			if net.ParseIP(gateway) == nil || !strings.Contains(gateway, ".") {
				return fmt.Errorf("invalid --gateway %q: must be an IPv4 address", gateway)
			}
		}
	}
	if dryRun && yes {
		return fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !dryRun && !yes {
		return fmt.Errorf("network set-ip modifies camera settings; pass --yes to apply or --dry-run to preview")
	}
	return nil
}

func runNetworkSetIP(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkSetIPFormat); err != nil {
		return err
	}
	if err := validateSetIPArgs(networkSetIPInterface, networkSetIPAddress, networkSetIPGateway,
		networkSetIPPrefixLen, networkSetIPDHCP, networkSetIPDryRun, networkSetIPYes); err != nil {
		return err
	}

	pw, err := resolvePassword(networkSetIPFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkSetIPFlags.ip, networkSetIPFlags.port, networkSetIPFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Fetch current state for dry-run display
	ifaces, _ := client.GetNetworkInterfaces(ctx)
	gateways, _ := client.GetNetworkGateway(ctx)

	cfg := camera.SetIPConfig{
		InterfaceToken: networkSetIPInterface,
		DHCP:           networkSetIPDHCP,
		Address:        networkSetIPAddress,
		PrefixLength:   networkSetIPPrefixLen,
		Gateway:        networkSetIPGateway,
	}

	if networkSetIPDryRun {
		printSetIPPreview(ifaces, gateways, cfg, networkSetIPFormat)
		return nil
	}

	fmt.Fprintln(os.Stderr, "WARNING: Changing network settings may make the camera unreachable.")

	result, err := client.SetNetworkIP(ctx, cfg)
	if err != nil {
		return err
	}

	if networkSetIPFormat == formatJSON {
		return writeJSON(struct {
			Applied      bool   `json:"applied"`
			RebootNeeded bool   `json:"reboot_needed"`
			Interface    string `json:"interface"`
			Mode         string `json:"mode"`
			Address      string `json:"address,omitempty"`
			PrefixLength int    `json:"prefix_length,omitempty"`
			Gateway      string `json:"gateway,omitempty"`
		}{
			Applied:      true,
			RebootNeeded: result.RebootNeeded,
			Interface:    cfg.InterfaceToken,
			Mode:         ipMode(cfg.DHCP),
			Address:      cfg.Address,
			PrefixLength: cfg.PrefixLength,
			Gateway:      cfg.Gateway,
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "applied\tyes\n")
	fmt.Fprintf(w, "interface\t%s\n", cfg.InterfaceToken)
	fmt.Fprintf(w, "mode\t%s\n", ipMode(cfg.DHCP))
	if !cfg.DHCP {
		fmt.Fprintf(w, "address\t%s/%d\n", cfg.Address, cfg.PrefixLength)
		if cfg.Gateway != "" {
			fmt.Fprintf(w, "gateway\t%s\n", cfg.Gateway)
		}
	}
	if result.RebootNeeded {
		fmt.Fprintf(w, "reboot_needed\tyes\n")
	}
	w.Flush()
	if result.RebootNeeded {
		fmt.Fprintln(os.Stderr, "NOTE: Camera reported that a reboot is required for changes to take effect.")
	}
	return nil
}

func printSetIPPreview(ifaces []camera.NetworkInterfaceInfo, gateways []string, cfg camera.SetIPConfig, format string) {
	currentMode := "unknown"
	currentAddr := ""
	currentGW := ""
	for _, iface := range ifaces {
		if iface.Token == cfg.InterfaceToken {
			if iface.IPv4DHCP {
				currentMode = "dhcp"
				if iface.IPv4FromDHCP != nil {
					currentAddr = iface.IPv4FromDHCP.String()
				}
			} else {
				currentMode = "static"
				for _, m := range iface.IPv4Manual {
					currentAddr = m.String()
				}
			}
			break
		}
	}
	if len(gateways) > 0 {
		currentGW = gateways[0]
	}

	requestedAddr := ""
	if !cfg.DHCP {
		requestedAddr = fmt.Sprintf("%s/%d", cfg.Address, cfg.PrefixLength)
	}
	requestedGW := cfg.Gateway

	if format == formatJSON {
		_ = writeJSON(struct {
			DryRun    bool   `json:"dry_run"`
			Interface string `json:"interface"`
			Current   struct {
				Mode    string `json:"mode"`
				Address string `json:"address,omitempty"`
				Gateway string `json:"gateway,omitempty"`
			} `json:"current"`
			Requested struct {
				Mode    string `json:"mode"`
				Address string `json:"address,omitempty"`
				Gateway string `json:"gateway,omitempty"`
			} `json:"requested"`
		}{
			DryRun:    true,
			Interface: cfg.InterfaceToken,
			Current: struct {
				Mode    string `json:"mode"`
				Address string `json:"address,omitempty"`
				Gateway string `json:"gateway,omitempty"`
			}{Mode: currentMode, Address: currentAddr, Gateway: currentGW},
			Requested: struct {
				Mode    string `json:"mode"`
				Address string `json:"address,omitempty"`
				Gateway string `json:"gateway,omitempty"`
			}{Mode: ipMode(cfg.DHCP), Address: requestedAddr, Gateway: requestedGW},
		})
		return
	}

	fmt.Fprintln(os.Stderr, "WARNING: Changing network settings may make the camera unreachable.")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tCURRENT\tREQUESTED")
	fmt.Fprintf(w, "interface\t%s\t%s\n", cfg.InterfaceToken, cfg.InterfaceToken)
	fmt.Fprintf(w, "mode\t%s\t%s\n", currentMode, ipMode(cfg.DHCP))
	if !cfg.DHCP {
		fmt.Fprintf(w, "address\t%s\t%s\n", currentAddr, requestedAddr)
		fmt.Fprintf(w, "gateway\t%s\t%s\n", currentGW, requestedGW)
	}
	w.Flush()
	fmt.Println()
	fmt.Println("Dry run: no changes applied.")
}

func ipMode(dhcp bool) string {
	if dhcp {
		return "dhcp"
	}
	return "static"
}
