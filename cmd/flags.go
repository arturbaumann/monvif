package cmd

// cameraFlags holds flags shared across camera subcommands.
type cameraFlags struct {
	ip       string
	port     int
	user     string
	password string
}
