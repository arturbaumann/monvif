package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var debugFlag bool

var rootCmd = &cobra.Command{
	Use:   "monvif",
	Short: "ONVIF camera discovery and querying tool",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debugFlag {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "show RPC debug logs")

	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(capabilitiesCmd)
	rootCmd.AddCommand(profilesCmd)
	rootCmd.AddCommand(streamURICmd)
	rootCmd.AddCommand(snapshotURICmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(imagingCmd)
	rootCmd.AddCommand(networkCmd)
}
