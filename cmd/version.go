package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(versionString())
	},
}

func versionString() string {
	return fmt.Sprintf("monvif %s (commit %s, built %s, %s)", version, commit, date, runtime.Version())
}
