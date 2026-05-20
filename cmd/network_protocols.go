package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/artur/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var networkProtocolsCmd = &cobra.Command{
	Use:   "protocols",
	Short: "List camera network protocols",
	RunE:  runNetworkProtocols,
}

var (
	networkProtoFlags  cameraFlags
	networkProtoFormat string
)

func init() {
	networkProtocolsCmd.Flags().StringVar(&networkProtoFlags.ip, "ip", "", "camera IP address (required)")
	networkProtocolsCmd.Flags().IntVar(&networkProtoFlags.port, "port", 80, "ONVIF port")
	networkProtocolsCmd.Flags().StringVar(&networkProtoFlags.user, "user", "", "username (required)")
	networkProtocolsCmd.Flags().StringVar(&networkProtoFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	networkProtocolsCmd.MarkFlagRequired("ip")
	networkProtocolsCmd.MarkFlagRequired("user")
	addFormatFlag(networkProtocolsCmd, &networkProtoFormat, formatTable)
}

func runNetworkProtocols(cmd *cobra.Command, args []string) error {
	if err := validateFormat(networkProtoFormat); err != nil {
		return err
	}
	pw, err := resolvePassword(networkProtoFlags.password)
	if err != nil {
		return err
	}
	client, err := camera.New(networkProtoFlags.ip, networkProtoFlags.port, networkProtoFlags.user, pw)
	if err != nil {
		return err
	}

	protos, err := client.GetNetworkProtocols(context.Background())
	if err != nil {
		return err
	}

	if networkProtoFormat == formatJSON {
		if protos == nil {
			protos = []camera.NetworkProtocolInfo{}
		}
		return writeJSON(protos)
	}

	if len(protos) == 0 {
		fmt.Fprintln(os.Stdout, "No network protocols reported by camera.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROTOCOL\tENABLED\tPORTS")
	for _, p := range protos {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, boolMark(p.Enabled), portList(p.Ports))
	}
	w.Flush()
	return nil
}

func portList(ports []int) string {
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = strconv.Itoa(p)
	}
	return strings.Join(strs, ",")
}
