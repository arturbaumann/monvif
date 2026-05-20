package cmd

import "github.com/spf13/cobra"

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Query and update camera network configuration",
}

func init() {
	networkCmd.AddCommand(networkGetCmd)
	networkCmd.AddCommand(networkInterfacesCmd)
	networkCmd.AddCommand(networkProtocolsCmd)
	networkCmd.AddCommand(networkDNSCmd)
	networkCmd.AddCommand(networkNTPCmd)
	networkCmd.AddCommand(networkHostnameCmd)
	networkCmd.AddCommand(networkSetIPCmd)
	networkCmd.AddCommand(networkSetDNSCmd)
	networkCmd.AddCommand(networkSetNTPCmd)
	networkCmd.AddCommand(networkSetHostnameCmd)
}
