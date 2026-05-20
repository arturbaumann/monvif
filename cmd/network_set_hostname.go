package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkSetHostnameCmd = &cobra.Command{
	Use:   "set-hostname",
	Short: "Set camera hostname",
	RunE:  runNetworkSetHostname,
}

var (
	networkSetHostnameFlags  cameraFlags
	networkSetHostnameName   string
	networkSetHostnameDryRun bool
	networkSetHostnameYes    bool
	networkSetHostnameFormat string
)

// hostnameRe accepts RFC 952/1123 hostnames: letters, digits, hyphens; no leading/trailing hyphen.
var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)

func init() {
	networkSetHostnameCmd.Flags().StringVar(&networkSetHostnameFlags.ip, "ip", "", "camera IP address (required)")
	networkSetHostnameCmd.Flags().IntVar(&networkSetHostnameFlags.port, "port", 80, "ONVIF port")
	networkSetHostnameCmd.Flags().StringVar(&networkSetHostnameFlags.user, "user", "", "username (required)")
	networkSetHostnameCmd.Flags().StringVar(&networkSetHostnameFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkSetHostnameCmd.Flags().StringVar(&networkSetHostnameName, "name", "", "hostname to set (required)")
	networkSetHostnameCmd.Flags().BoolVar(&networkSetHostnameDryRun, "dry-run", false, "preview change without applying")
	networkSetHostnameCmd.Flags().BoolVar(&networkSetHostnameYes, "yes", false, "apply changes to the camera")
	networkSetHostnameCmd.MarkFlagRequired("ip")
	networkSetHostnameCmd.MarkFlagRequired("user")
	networkSetHostnameCmd.MarkFlagRequired("name")
	addFormatFlag(networkSetHostnameCmd, &networkSetHostnameFormat, formatTable)
}

func validateSetHostnameArgs(name string, dryRun, yes bool) error {
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("hostname too long (max 63 characters)")
	}
	if !hostnameRe.MatchString(name) {
		return fmt.Errorf("invalid hostname %q: use letters, digits, hyphens; no leading/trailing hyphen", name)
	}
	if dryRun && yes {
		return fmt.Errorf("--dry-run and --yes are mutually exclusive")
	}
	if !dryRun && !yes {
		return fmt.Errorf("network set-hostname modifies camera settings; pass --yes to apply or --dry-run to preview")
	}
	return nil
}

func runNetworkSetHostname(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkSetHostnameFormat); err != nil {
		return err
	}
	if err := validateSetHostnameArgs(networkSetHostnameName, networkSetHostnameDryRun, networkSetHostnameYes); err != nil {
		return err
	}

	pw, err := resolvePassword(networkSetHostnameFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkSetHostnameFlags.ip, networkSetHostnameFlags.port, networkSetHostnameFlags.user, pw)
	if err != nil {
		return err
	}

	ctx := context.Background()
	current, _ := client.GetHostnameInfo(ctx)

	if networkSetHostnameDryRun {
		printSetHostnamePreview(current, networkSetHostnameName, networkSetHostnameFormat)
		return nil
	}

	if err := client.SetHostname(ctx, networkSetHostnameName); err != nil {
		return err
	}

	if networkSetHostnameFormat == formatJSON {
		return writeJSON(struct {
			Applied bool   `json:"applied"`
			Before  string `json:"before"`
			After   string `json:"after"`
		}{Applied: true, Before: current.Name, After: networkSetHostnameName})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "applied\tyes\n")
	fmt.Fprintf(w, "before\t%s\n", current.Name)
	fmt.Fprintf(w, "after\t%s\n", networkSetHostnameName)
	w.Flush()
	return nil
}

func printSetHostnamePreview(current camera.HostnameInfo, requested, format string) {
	if format == formatJSON {
		_ = writeJSON(struct {
			DryRun    bool   `json:"dry_run"`
			Before    string `json:"before"`
			Requested string `json:"requested"`
		}{DryRun: true, Before: current.Name, Requested: requested})
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tCURRENT\tREQUESTED")
	fmt.Fprintf(w, "hostname\t%s\t%s\n", current.Name, requested)
	w.Flush()
	fmt.Println()
	fmt.Println("Dry run: no changes applied.")
}
