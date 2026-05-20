package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

const (
	formatTable    = "table"
	formatJSON     = "json"
	formatMarkdown = "markdown"
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

func redactURICredentials(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil || u.User == nil {
		return rawURI
	}
	// Rebuild manually — url.User("***").String() would percent-encode the asterisks.
	out := u.Scheme + "://***@" + u.Host + u.EscapedPath()
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}
