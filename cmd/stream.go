package cmd

import "github.com/spf13/cobra"

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Query and probe RTSP stream profiles",
}

func init() {
	streamCmd.AddCommand(streamProfilesCmd)
}
