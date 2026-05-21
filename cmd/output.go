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

// redactPassword returns rawURI with the password replaced by [REDACTED] while
// keeping the username visible. Used for --debug output so the operator can
// see which user is being used without the password being logged.
// If the URI has no password (or cannot be parsed), rawURI is returned unchanged.
func redactPassword(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil || u.User == nil {
		return rawURI
	}
	if _, hasPass := u.User.Password(); !hasPass {
		return rawURI
	}
	// Rebuild manually to avoid percent-encoding the [REDACTED] marker.
	out := u.Scheme + "://" + u.User.Username() + ":[REDACTED]@" + u.Host + u.EscapedPath()
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}
