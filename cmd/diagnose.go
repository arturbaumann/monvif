package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/arturbaumann/monvif/internal/camera"
	"github.com/arturbaumann/monvif/internal/inventory"
	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run diagnostics on all cameras in a TSV inventory file",
	Long: `Runs a comprehensive health check on each camera in the inventory file.

Checks: TCP connectivity, ONVIF auth, device info, capabilities, media profiles,
stream URI, snapshot URI, imaging settings, and network configuration.

Per-camera status:
  OK   — all core checks passed
  WARN — core OK, but optional checks (snapshot/imaging/network) had issues
  FAIL — TCP unreachable, auth failed, no media profiles, or no stream URI

Diagnostics are read-only — no camera settings are changed.
Password is read from MONVIF_PASSWORD. Do not store passwords in the TSV file.`,
	RunE: runDiagnose,
}

var (
	diagnoseFile          string
	diagnoseUser          string
	diagnoseFormat        string
	diagnoseOutput        string
	diagnoseOnly          string
	diagnoseTimeout       time.Duration
	diagnoseCameraTimeout time.Duration
	diagnoseConcurrency   int
	diagnoseProgress      string
	diagnoseRTSPPort      int
	diagnoseQuick         bool
	diagnoseSkipDisc      bool
	diagnoseSkipSnapshot  bool
	diagnoseSkipImaging   bool
	diagnoseSkipNetwork   bool
	diagnoseSkipRTSP      bool
)

func init() {
	diagnoseCmd.Flags().StringVar(&diagnoseFile, "file", "", "path to TSV inventory file (required)")
	diagnoseCmd.Flags().StringVar(&diagnoseUser, "user", "", "username (required)")
	diagnoseCmd.Flags().StringVar(&diagnoseFormat, "format", formatTable, "output format: table, json, or markdown")
	diagnoseCmd.Flags().StringVar(&diagnoseOutput, "output", "", "write report to this file (stdout if omitted)")
	diagnoseCmd.Flags().StringVar(&diagnoseOnly, "only", "", "diagnose only the camera matching this name or IP")
	diagnoseCmd.Flags().DurationVar(&diagnoseTimeout, "timeout", 5*time.Second, "TCP dial timeout per low-level check")
	diagnoseCmd.Flags().DurationVar(&diagnoseCameraTimeout, "camera-timeout", 60*time.Second, "max total time per active camera")
	diagnoseCmd.Flags().IntVar(&diagnoseConcurrency, "concurrency", 4, "number of cameras to diagnose concurrently")
	diagnoseCmd.Flags().StringVar(&diagnoseProgress, "progress", progressAuto, "progress display: auto, always, or never")
	diagnoseCmd.Flags().IntVar(&diagnoseRTSPPort, "rtsp-port", 554, "RTSP port to test")
	diagnoseCmd.Flags().BoolVar(&diagnoseQuick, "quick", false, "skip snapshot, imaging, network, and RTSP checks (recommended first run)")
	diagnoseCmd.Flags().BoolVar(&diagnoseSkipDisc, "skip-discovery", true, "skip WS-Discovery check")
	diagnoseCmd.Flags().BoolVar(&diagnoseSkipSnapshot, "skip-snapshot", false, "skip snapshot URI check")
	diagnoseCmd.Flags().BoolVar(&diagnoseSkipImaging, "skip-imaging", false, "skip imaging settings check")
	diagnoseCmd.Flags().BoolVar(&diagnoseSkipNetwork, "skip-network", false, "skip network configuration check")
	diagnoseCmd.Flags().BoolVar(&diagnoseSkipRTSP, "skip-rtsp", false, "skip TCP RTSP port check")
	diagnoseCmd.MarkFlagRequired("file")
	diagnoseCmd.MarkFlagRequired("user")
}

func validateDiagnoseFormat(f string) error {
	switch f {
	case formatTable, formatJSON, formatMarkdown:
		return nil
	default:
		return fmt.Errorf("invalid --format %q: must be table, json, or markdown", f)
	}
}

func validateDiagnoseConcurrency(n int) error {
	if n <= 0 {
		return fmt.Errorf("invalid --concurrency %d: must be at least 1", n)
	}
	return nil
}

// diagProgressTracker provides a thread-safe progress display for concurrent
// camera diagnostics. It tracks which cameras are active and what check they
// are currently running, then renders a single-line status to the reporter.
type diagProgressTracker struct {
	mu     sync.Mutex
	stages map[string]string // camera name → current check name
	done   int
	total  int
	start  time.Time
	pr     *progressReporter
}

func newDiagProgressTracker(total int, start time.Time, pr *progressReporter) *diagProgressTracker {
	return &diagProgressTracker{
		stages: make(map[string]string),
		total:  total,
		start:  start,
		pr:     pr,
	}
}

// stageCallback satisfies the camera.DiagnosticsOptions.Stage signature.
// A stage of "done" removes the camera from the active set and increments done.
func (pt *diagProgressTracker) stageCallback(_, _ int, name, stage string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if stage == "done" {
		delete(pt.stages, name)
		pt.done++
	} else {
		pt.stages[name] = stage
	}

	var running []string
	for n, s := range pt.stages {
		running = append(running, n+":"+s)
	}
	sort.Strings(running)

	elapsed := time.Since(pt.start).Seconds()
	msg := fmt.Sprintf("Diagnosing %d/%d  elapsed %.1fs", pt.done, pt.total, elapsed)
	if len(running) > 0 {
		msg += "  " + strings.Join(running, "  ")
	}
	pt.pr.update(msg)
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	if err := validateDiagnoseFormat(diagnoseFormat); err != nil {
		return err
	}
	if err := validateProgress(diagnoseProgress); err != nil {
		return err
	}
	if err := validateDiagnoseConcurrency(diagnoseConcurrency); err != nil {
		return err
	}

	pw, err := resolvePassword("")
	if err != nil {
		return err
	}

	f, err := os.Open(diagnoseFile)
	if err != nil {
		return fmt.Errorf("opening inventory file: %w", err)
	}
	defer f.Close()

	cams, err := inventory.ParseTSV(f)
	if err != nil {
		return fmt.Errorf("parsing inventory file: %w", err)
	}
	if len(cams) == 0 {
		fmt.Fprintln(os.Stderr, "No cameras found in inventory file.")
		return nil
	}

	// --only filters to a single camera by name or IP.
	if diagnoseOnly != "" {
		var filtered []inventory.Camera
		for _, c := range cams {
			if c.Name == diagnoseOnly || c.IP == diagnoseOnly {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("--only %q: no camera matching name or IP found in %s", diagnoseOnly, diagnoseFile)
		}
		cams = filtered
	}

	targets := make([]camera.DiagnosticsTarget, len(cams))
	for i, c := range cams {
		targets[i] = camera.DiagnosticsTarget{Name: c.Name, IP: c.IP, Port: c.Port}
	}

	// Treat markdown like table for auto-progress (both are non-JSON).
	progressFmt := diagnoseFormat
	if progressFmt == formatMarkdown {
		progressFmt = formatTable
	}
	pr := newProgressReporter(diagnoseProgress, progressFmt, debugFlag)
	runStart := time.Now()
	tracker := newDiagProgressTracker(len(cams), runStart, pr)

	skipSnapshot := diagnoseSkipSnapshot || diagnoseQuick
	skipImaging := diagnoseSkipImaging || diagnoseQuick
	skipNetwork := diagnoseSkipNetwork || diagnoseQuick
	skipRTSP := diagnoseSkipRTSP || diagnoseQuick

	opts := camera.DiagnosticsOptions{
		Username:      diagnoseUser,
		Password:      pw,
		Timeout:       diagnoseTimeout,
		CameraTimeout: diagnoseCameraTimeout,
		Concurrency:   diagnoseConcurrency,
		RTSPPort:      diagnoseRTSPPort,
		Debug:         debugFlag,
		SkipDiscovery: diagnoseSkipDisc,
		SkipSnapshot:  skipSnapshot,
		SkipImaging:   skipImaging,
		SkipNetwork:   skipNetwork,
		SkipRTSP:      skipRTSP,
		RedactURI:     redactURICredentials,
		Stage:         tracker.stageCallback,
	}

	report := camera.RunDiagnostics(context.Background(), targets, opts)
	pr.clear()

	// Resolve output writer.
	var outW io.Writer = os.Stdout
	var outPath string
	if diagnoseOutput != "" {
		outPath = diagnoseOutput
		of, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer of.Close()
		outW = of
	}

	switch diagnoseFormat {
	case formatJSON:
		if err := renderDiagnoseJSON(report, outW); err != nil {
			return err
		}
	case formatMarkdown:
		renderDiagnoseMarkdown(report, outW)
	default:
		renderDiagnoseTable(report, outW)
	}

	if outPath != "" {
		fmt.Fprintf(os.Stdout, "Wrote diagnostics report to %s\n", outPath)
	}
	return nil
}

// ── table renderer ────────────────────────────────────────────────────────────

func renderDiagnoseTable(r camera.DiagnosticsReport, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATUS\tTCP\tAUTH\tPROFILES\tSTREAM\tSNAPSHOT\tIMAGING\tNETWORK\tNOTES")
	for _, d := range r.Cameras {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Name,
			string(d.Status),
			diagCheckCol(d, "tcp-onvif"),
			diagCheckCol(d, "auth"),
			diagProfilesCol(d),
			diagCheckCol(d, "stream-uri"),
			diagCheckCol(d, "snapshot-uri"),
			diagCheckCol(d, "imaging"),
			diagCheckCol(d, "network"),
			diagNotesCol(d, 60),
		)
	}
	tw.Flush()
	fmt.Fprintf(w, "\nTotal: %d  OK: %d  WARN: %d  FAIL: %d  (%.1fs)\n",
		r.Totals.Total, r.Totals.OK, r.Totals.Warn, r.Totals.Fail,
		float64(r.DurationMS)/1000)
}

// ── JSON renderer ─────────────────────────────────────────────────────────────

func renderDiagnoseJSON(r camera.DiagnosticsReport, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ── markdown renderer ─────────────────────────────────────────────────────────

func renderDiagnoseMarkdown(r camera.DiagnosticsReport, w io.Writer) {
	fmt.Fprintln(w, "# monvif Diagnostic Report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Generated: %s  \n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Duration: %.1fs\n", float64(r.DurationMS)/1000)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Summary")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Status | Count |")
	fmt.Fprintln(w, "|--------|-------|")
	fmt.Fprintf(w, "| OK     | %d |\n", r.Totals.OK)
	fmt.Fprintf(w, "| WARN   | %d |\n", r.Totals.Warn)
	fmt.Fprintf(w, "| FAIL   | %d |\n", r.Totals.Fail)
	fmt.Fprintf(w, "| **Total** | **%d** |\n", r.Totals.Total)
	fmt.Fprintln(w)

	for _, d := range r.Cameras {
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "## %s (%s:%d) — %s\n", d.Name, d.IP, d.Port, string(d.Status))
		fmt.Fprintln(w)

		if d.DeviceInfo != nil {
			di := d.DeviceInfo
			fmt.Fprintln(w, "| Manufacturer | Model | Firmware | Serial |")
			fmt.Fprintln(w, "|-------------|-------|----------|--------|")
			fmt.Fprintf(w, "| %s | %s | %s | %s |\n",
				mdEsc(di.Manufacturer), mdEsc(di.Model), mdEsc(di.FirmwareVersion), mdEsc(di.SerialNumber))
			fmt.Fprintln(w)
		}

		if len(d.Checks) > 0 {
			fmt.Fprintln(w, "### Checks")
			fmt.Fprintln(w)
			fmt.Fprintln(w, "| Check | Status | Message | ms |")
			fmt.Fprintln(w, "|-------|--------|---------|----|")
			for _, c := range d.Checks {
				fmt.Fprintf(w, "| %s | %s | %s | %d |\n",
					c.Name, string(c.Status), mdEsc(c.Message), c.DurationMS)
			}
			fmt.Fprintln(w)
		}

		if d.Network != nil {
			fmt.Fprintln(w, "### Network")
			fmt.Fprintln(w)
			nr := d.Network
			if nr.Hostname != "" {
				fmt.Fprintf(w, "- Hostname: %s\n", nr.Hostname)
			}
			if nr.IPv4Address != "" {
				fmt.Fprintf(w, "- IPv4: %s\n", nr.IPv4Address)
			}
			fmt.Fprintf(w, "- IPv4 DHCP: %s\n", boolWord(nr.IPv4DHCP))
			if nr.MAC != "" {
				fmt.Fprintf(w, "- MAC: %s\n", nr.MAC)
			}
			if nr.Gateway != "" {
				fmt.Fprintf(w, "- Gateway: %s\n", nr.Gateway)
			}
			if nr.DNSServers != "" {
				fmt.Fprintf(w, "- DNS: %s\n", nr.DNSServers)
			}
			if nr.NTPServers != "" {
				fmt.Fprintf(w, "- NTP: %s\n", nr.NTPServers)
			}
			fmt.Fprintln(w)
		}

		if len(d.Warnings) > 0 {
			fmt.Fprintln(w, "### Warnings")
			fmt.Fprintln(w)
			for _, ww := range d.Warnings {
				fmt.Fprintf(w, "- %s\n", ww)
			}
			fmt.Fprintln(w)
		}

		if len(d.Errors) > 0 {
			fmt.Fprintln(w, "### Errors")
			fmt.Fprintln(w)
			for _, e := range d.Errors {
				fmt.Fprintf(w, "- %s\n", e)
			}
			fmt.Fprintln(w)
		}
	}
}

// ── column helpers ────────────────────────────────────────────────────────────

func diagCheckStatusOf(d camera.CameraDiagnostic, name string) camera.CheckStatus {
	for _, c := range d.Checks {
		if c.Name == name {
			return c.Status
		}
	}
	return ""
}

func diagCheckCol(d camera.CameraDiagnostic, name string) string {
	switch diagCheckStatusOf(d, name) {
	case camera.CheckOK:
		return "yes"
	case camera.CheckFail:
		return "no"
	case camera.CheckWarn:
		return "warn"
	case camera.CheckSkipped:
		return "skip"
	default:
		return "-"
	}
}

func diagProfilesCol(d camera.CameraDiagnostic) string {
	if d.Profiles != nil {
		return fmt.Sprintf("%d", d.Profiles.Count)
	}
	if diagCheckStatusOf(d, "profiles") == camera.CheckFail {
		return "no"
	}
	return "-"
}

func diagNotesCol(d camera.CameraDiagnostic, maxLen int) string {
	if len(d.Errors) > 0 {
		return truncateDiagStr(d.Errors[0], maxLen)
	}
	// Report any check that took more than 2s as slow.
	var slowNotes []string
	for _, c := range d.Checks {
		if c.DurationMS >= 2000 {
			slowNotes = append(slowNotes, fmt.Sprintf("slow:%s(%dms)", c.Name, c.DurationMS))
		}
	}
	if len(slowNotes) > 0 {
		return truncateDiagStr(strings.Join(slowNotes, " "), maxLen)
	}
	if len(d.Warnings) > 0 {
		return truncateDiagStr(d.Warnings[0], maxLen)
	}
	return ""
}

func truncateDiagStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func mdEsc(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
