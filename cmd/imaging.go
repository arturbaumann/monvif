package cmd

import "github.com/spf13/cobra"

var imagingCmd = &cobra.Command{
	Use:   "imaging",
	Short: "Query and update camera imaging settings",
}

func init() {
	imagingCmd.AddCommand(imagingGetCmd)
	imagingCmd.AddCommand(imagingSetCmd)
}
