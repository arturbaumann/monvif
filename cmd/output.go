package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	formatTable = "table"
	formatJSON  = "json"
)

func addFormatFlag(cmd *cobra.Command, target *string, defaultVal string) {
	cmd.Flags().StringVar(target, "format", defaultVal, "output format: table or json")
}

func validateFormat(f string) error {
	if f != formatTable && f != formatJSON {
		return fmt.Errorf("invalid --format %q: must be table or json", f)
	}
	return nil
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
