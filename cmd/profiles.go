package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List media profiles on an ONVIF camera",
	RunE:  runProfiles,
}

var (
	profilesFlags  cameraFlags
	profilesFormat string
)

func init() {
	profilesCmd.Flags().StringVar(&profilesFlags.ip, "ip", "", "camera IP address (required)")
	profilesCmd.Flags().IntVar(&profilesFlags.port, "port", 80, "ONVIF port")
	profilesCmd.Flags().StringVar(&profilesFlags.user, "user", "", "username (required)")
	profilesCmd.Flags().StringVar(&profilesFlags.password, "password", "", "password (or set MONVIF_PASSWORD)")
	profilesCmd.MarkFlagRequired("ip")
	profilesCmd.MarkFlagRequired("user")
	addFormatFlag(profilesCmd, &profilesFormat, formatTable)
}

type profileJSON struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

func runProfiles(cmd *cobra.Command, args []string) error {
	if err := validateFormat(profilesFormat); err != nil {
		return err
	}

	pw, err := resolvePassword(profilesFlags.password)
	if err != nil {
		return err
	}

	client, err := camera.New(profilesFlags.ip, profilesFlags.port, profilesFlags.user, pw)
	if err != nil {
		return err
	}

	profiles, err := client.GetProfiles(context.Background())
	if err != nil {
		return err
	}

	if profilesFormat == formatJSON {
		out := make([]profileJSON, len(profiles))
		for i, p := range profiles {
			out[i] = profileJSON{Token: p.Token, Name: p.Name}
		}
		return writeJSON(out)
	}

	if len(profiles) == 0 {
		fmt.Println("No media profiles found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOKEN\tNAME")
	for _, p := range profiles {
		fmt.Fprintf(w, "%s\t%s\n", p.Token, p.Name)
	}
	w.Flush()
	return nil
}
