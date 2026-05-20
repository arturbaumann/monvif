package cmd

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

// resolvePassword returns the password to use, in priority order:
//  1. --password flag
//  2. MONVIF_PASSWORD env var
//  3. Interactive prompt (when stdin is a terminal)
func resolvePassword(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv("MONVIF_PASSWORD"); env != "" {
		return env, nil
	}
	if !term.IsTerminal(int(syscall.Stdin)) {
		return "", fmt.Errorf("password required: set --password or MONVIF_PASSWORD")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	raw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(raw), nil
}
