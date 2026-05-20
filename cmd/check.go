package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/artur/monvif/internal/camera"
	"github.com/artur/monvif/internal/inventory"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check all cameras listed in a TSV inventory file",
	Long: `Reads a TSV file (name<TAB>ip<TAB>port) and queries each camera.
Password is read from MONVIF_PASSWORD. Do not store passwords in the TSV file.`,
	RunE: runCheck,
}

var (
	checkFile     string
	checkUser     string
	checkFormat   string
	checkCaps     bool
	checkProgress string
)

func init() {
	checkCmd.Flags().StringVar(&checkFile, "file", "", "path to TSV inventory file (required)")
	checkCmd.Flags().StringVar(&checkUser, "user", "", "username (required)")
	checkCmd.Flags().BoolVar(&checkCaps, "capabilities", false, "also query capability URLs (DEVICE_URL, MEDIA_URL, IMAGING_URL, EVENTS_URL, PTZ_URL)")
	checkCmd.Flags().StringVar(&checkProgress, "progress", progressAuto, "progress display: auto, always, or never")
	checkCmd.MarkFlagRequired("file")
	checkCmd.MarkFlagRequired("user")
	addFormatFlag(checkCmd, &checkFormat, formatTable)
}

// checkCapsJSON is the nested capabilities object in JSON output.
type checkCapsJSON struct {
	DeviceURL  string `json:"device_url,omitempty"`
	MediaURL   string `json:"media_url,omitempty"`
	ImagingURL string `json:"imaging_url,omitempty"`
	EventsURL  string `json:"events_url,omitempty"`
	PTZURL     string `json:"ptz_url,omitempty"`
}

type checkJSON struct {
	Name          string         `json:"name"`
	IP            string         `json:"ip"`
	Port          int            `json:"port"`
	Reachable     bool           `json:"reachable"`
	Authenticated bool           `json:"authenticated"`
	Manufacturer  string         `json:"manufacturer,omitempty"`
	Model         string         `json:"model,omitempty"`
	Firmware      string         `json:"firmware,omitempty"`
	SerialNumber  string         `json:"serial_number,omitempty"`
	HardwareID    string         `json:"hardware_id,omitempty"`
	Capabilities  *checkCapsJSON `json:"capabilities,omitempty"`
	Error         string         `json:"error,omitempty"`
}

func toCheckJSON(r camera.CheckResult) checkJSON {
	j := checkJSON{
		Name:          r.Name,
		IP:            r.IP,
		Port:          r.Port,
		Reachable:     r.Reachable,
		Authenticated: r.Authenticated,
		Manufacturer:  r.Info.Manufacturer,
		Model:         r.Info.Model,
		Firmware:      r.Info.FirmwareVersion,
		SerialNumber:  r.Info.SerialNumber,
		HardwareID:    r.Info.HardwareID,
		Error:         r.Err,
	}
	if r.Caps != nil {
		j.Capabilities = &checkCapsJSON{
			DeviceURL:  r.Caps.DeviceXAddr,
			MediaURL:   r.Caps.MediaXAddr,
			ImagingURL: r.Caps.ImagingXAddr,
			EventsURL:  r.Caps.EventsXAddr,
			PTZURL:     r.Caps.PTZXAddr,
		}
	}
	return j
}

func runCheck(cmd *cobra.Command, args []string) error {
	if err := validateFormat(checkFormat); err != nil {
		return err
	}
	if err := validateProgress(checkProgress); err != nil {
		return err
	}

	pw, err := resolvePassword("")
	if err != nil {
		return err
	}

	f, err := os.Open(checkFile)
	if err != nil {
		return fmt.Errorf("opening inventory file: %w", err)
	}
	defer f.Close()

	cameras, err := inventory.ParseTSV(f)
	if err != nil {
		return fmt.Errorf("parsing inventory file: %w", err)
	}
	if len(cameras) == 0 {
		fmt.Fprintln(os.Stderr, "No cameras found in inventory file.")
		return nil
	}

	pr := newProgressReporter(checkProgress, checkFormat, debugFlag)
	start := time.Now()
	total := len(cameras)

	var results []camera.CheckResult
	for i, cam := range cameras {
		idx := i + 1
		result := camera.Check(context.Background(), cam.Name, cam.IP, cam.Port, checkUser, pw, camera.CheckOptions{
			WithCapabilities: checkCaps,
			Stage: func(stage string) {
				pr.update(fmt.Sprintf("Checking cameras: %d/%d  %s  %s  %s  elapsed %.1fs",
					idx, total, cam.Name, cam.IP, stage, time.Since(start).Seconds()))
			},
		})
		results = append(results, result)
	}
	pr.clear()

	if checkFormat == formatJSON {
		out := make([]checkJSON, len(results))
		for i, r := range results {
			out[i] = toCheckJSON(r)
		}
		return writeJSON(out)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if checkCaps {
		fmt.Fprintln(w, "NAME\tIP\tPORT\tREACHABLE\tAUTHENTICATED\tMANUFACTURER\tMODEL\tFIRMWARE\tDEVICE_URL\tMEDIA_URL\tIMAGING_URL\tEVENTS_URL\tPTZ_URL\tERROR")
		for _, r := range results {
			var d, m, im, ev, ptz string
			if r.Caps != nil {
				d, m, im, ev, ptz = r.Caps.DeviceXAddr, r.Caps.MediaXAddr, r.Caps.ImagingXAddr, r.Caps.EventsXAddr, r.Caps.PTZXAddr
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Name, r.IP, r.Port,
				boolMark(r.Reachable), boolMark(r.Authenticated),
				r.Info.Manufacturer, r.Info.Model, r.Info.FirmwareVersion,
				d, m, im, ev, ptz,
				r.Err,
			)
		}
	} else {
		fmt.Fprintln(w, "NAME\tIP\tPORT\tREACHABLE\tAUTHENTICATED\tMANUFACTURER\tMODEL\tFIRMWARE\tERROR")
		for _, r := range results {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Name, r.IP, r.Port,
				boolMark(r.Reachable), boolMark(r.Authenticated),
				r.Info.Manufacturer, r.Info.Model, r.Info.FirmwareVersion,
				r.Err,
			)
		}
	}
	w.Flush()
	return nil
}

func boolMark(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
