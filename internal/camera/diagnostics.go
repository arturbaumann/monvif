package camera

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// DiagnosticStatus is the overall health of a camera.
type DiagnosticStatus string

const (
	StatusOK   DiagnosticStatus = "OK"
	StatusWarn DiagnosticStatus = "WARN"
	StatusFail DiagnosticStatus = "FAIL"
)

// CheckStatus is the outcome of one diagnostic step.
type CheckStatus string

const (
	CheckOK      CheckStatus = "ok"
	CheckWarn    CheckStatus = "warn"
	CheckFail    CheckStatus = "fail"
	CheckSkipped CheckStatus = "skipped"
)

// DiagnosticsTarget is one camera to diagnose.
type DiagnosticsTarget struct {
	Name string
	IP   string
	Port int
}

// DiagCheckResult is the result of one diagnostic step.
type DiagCheckResult struct {
	Name       string      `json:"name"`
	Status     CheckStatus `json:"status"`
	Message    string      `json:"message,omitempty"`
	DurationMS int64       `json:"duration_ms"`
}

// DeviceInfoResult holds camera identity fields.
type DeviceInfoResult struct {
	Manufacturer    string `json:"manufacturer,omitempty"`
	Model           string `json:"model,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
	SerialNumber    string `json:"serial_number,omitempty"`
	HardwareID      string `json:"hardware_id,omitempty"`
}

// CapabilitiesResult holds ONVIF service endpoint URLs.
type CapabilitiesResult struct {
	DeviceURL  string `json:"device_url,omitempty"`
	MediaURL   string `json:"media_url,omitempty"`
	ImagingURL string `json:"imaging_url,omitempty"`
	EventsURL  string `json:"events_url,omitempty"`
	PTZURL     string `json:"ptz_url,omitempty"`
}

// ProfilesResult summarises available media profiles.
type ProfilesResult struct {
	Count  int      `json:"count"`
	Tokens []string `json:"tokens,omitempty"`
	Names  []string `json:"names,omitempty"`
}

// StreamURIResult holds a resolved stream URI (credentials redacted).
type StreamURIResult struct {
	ProfileToken string `json:"profile_token,omitempty"`
	ProfileName  string `json:"profile_name,omitempty"`
	URI          string `json:"uri,omitempty"`
}

// SnapshotURIResult holds a resolved snapshot URI (credentials redacted).
type SnapshotURIResult struct {
	ProfileToken string `json:"profile_token,omitempty"`
	ProfileName  string `json:"profile_name,omitempty"`
	URI          string `json:"uri,omitempty"`
}

// ImagingDiagResult holds imaging settings read from the camera.
type ImagingDiagResult struct {
	Brightness float64 `json:"brightness"`
	Contrast   float64 `json:"contrast"`
	Saturation float64 `json:"saturation"`
	Sharpness  float64 `json:"sharpness"`
}

// NetworkDiagResult holds key network settings.
type NetworkDiagResult struct {
	Hostname    string `json:"hostname,omitempty"`
	IfaceToken  string `json:"iface_token,omitempty"`
	IfaceName   string `json:"iface_name,omitempty"`
	MAC         string `json:"mac,omitempty"`
	IPv4DHCP    bool   `json:"ipv4_dhcp"`
	IPv4Address string `json:"ipv4_address,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	DNSFromDHCP bool   `json:"dns_from_dhcp"`
	DNSServers  string `json:"dns_servers,omitempty"`
	NTPFromDHCP bool   `json:"ntp_from_dhcp"`
	NTPServers  string `json:"ntp_servers,omitempty"`
}

// CameraDiagnostic is the full result for one camera.
type CameraDiagnostic struct {
	Name         string              `json:"name"`
	IP           string              `json:"ip"`
	Port         int                 `json:"port"`
	Status       DiagnosticStatus    `json:"status"`
	DurationMS   int64               `json:"duration_ms"`
	Checks       []DiagCheckResult   `json:"checks"`
	DeviceInfo   *DeviceInfoResult   `json:"device_info,omitempty"`
	Capabilities *CapabilitiesResult `json:"capabilities,omitempty"`
	Profiles     *ProfilesResult     `json:"profiles,omitempty"`
	StreamURI    *StreamURIResult    `json:"stream_uri,omitempty"`
	SnapshotURI  *SnapshotURIResult  `json:"snapshot_uri,omitempty"`
	Imaging      *ImagingDiagResult  `json:"imaging,omitempty"`
	Network      *NetworkDiagResult  `json:"network,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
	Errors       []string            `json:"errors,omitempty"`
}

// DiagnosticSummary is the per-status count breakdown.
type DiagnosticSummary struct {
	Total int `json:"total"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Fail  int `json:"fail"`
}

// DiagnosticsReport is the full output of a diagnostic run.
type DiagnosticsReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	DurationMS  int64              `json:"duration_ms"`
	Totals      DiagnosticSummary  `json:"totals"`
	Cameras     []CameraDiagnostic `json:"cameras"`
}

// DiagnosticsOptions configures a diagnostic run.
type DiagnosticsOptions struct {
	Username      string
	Password      string
	Timeout       time.Duration // TCP dial timeout per raw check; default 5s
	CameraTimeout time.Duration // max total time per active camera; default 30s
	Concurrency   int           // max cameras diagnosed concurrently; default 4
	RTSPPort      int           // default 554
	Debug         bool          // emit debug lines to stderr
	// Skip flags
	SkipDiscovery bool
	SkipSnapshot  bool
	SkipImaging   bool
	SkipNetwork   bool
	SkipRTSP      bool
	// Stage is called when a camera starts a check, and with stage="done" when
	// the camera finishes (success, fail, or timeout). Nil is safe.
	Stage     func(idx, total int, name, stage string)
	RedactURI func(string) string // credential redaction; nil = identity
}

// RunDiagnostics runs all checks for each target and returns the report.
//
// Cameras are diagnosed concurrently up to opts.Concurrency (default 4). The
// per-camera timeout (opts.CameraTimeout) starts only after a worker slot is
// acquired, so queued cameras cannot time out while waiting. Results are
// returned in the same order as targets regardless of completion order.
func RunDiagnostics(ctx context.Context, targets []DiagnosticsTarget, opts DiagnosticsOptions) DiagnosticsReport {
	start := time.Now()
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	if opts.CameraTimeout <= 0 {
		opts.CameraTimeout = 60 * time.Second
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.RTSPPort <= 0 {
		opts.RTSPPort = 554
	}
	if opts.RedactURI == nil {
		opts.RedactURI = func(s string) string { return s }
	}

	total := len(targets)
	results := make([]CameraDiagnostic, total)
	sem := make(chan struct{}, opts.Concurrency)

	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		localI, localT := i, t

		go func() {
			defer wg.Done()

			stageCall := func(s string) {
				if opts.Stage != nil {
					opts.Stage(localI+1, total, localT.Name, s)
				}
			}

			// Acquire a worker slot. Until we acquire, the camera has not
			// started — per-camera timeout must not tick yet.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[localI] = makeCancelledDiag(localT, time.Now())
				return
			}
			defer func() { <-sem }()

			// Timeout clock starts here, after acquiring the slot.
			camStart := time.Now()
			ps := new(diagPartialState)
			ch := make(chan CameraDiagnostic, 1)
			go func() { ch <- diagCamera(ctx, localT, opts, stageCall, ps) }()

			select {
			case d := <-ch:
				results[localI] = d
			case <-time.After(opts.CameraTimeout):
				results[localI] = makeTimeoutDiag(localT, camStart, opts.CameraTimeout, ps)
			case <-ctx.Done():
				results[localI] = makeCancelledDiag(localT, camStart)
			}

			// Signal completion so progress trackers can update.
			if opts.Stage != nil {
				opts.Stage(localI+1, total, localT.Name, "done")
			}
		}()
	}

	wg.Wait()

	report := DiagnosticsReport{GeneratedAt: start.UTC()}
	for _, d := range results {
		report.Cameras = append(report.Cameras, d)
		switch d.Status {
		case StatusOK:
			report.Totals.OK++
		case StatusWarn:
			report.Totals.Warn++
		default:
			report.Totals.Fail++
		}
	}
	report.Totals.Total = total
	report.DurationMS = time.Since(start).Milliseconds()
	return report
}

// diagPartialState accumulates check results as diagCamera progresses so that
// the goroutine wrapper can build a meaningful partial result on timeout.
// All methods are safe to call concurrently.
type diagPartialState struct {
	mu         sync.Mutex
	checks     []DiagCheckResult
	deviceInfo *DeviceInfoResult
	lastStage  string
}

func (ps *diagPartialState) addCheck(c DiagCheckResult) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.checks = append(ps.checks, c)
}

func (ps *diagPartialState) setDeviceInfo(di *DeviceInfoResult) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.deviceInfo = di
}

func (ps *diagPartialState) setStage(s string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastStage = s
}

func (ps *diagPartialState) snapshot() (checks []DiagCheckResult, di *DeviceInfoResult, lastStage string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	out := make([]DiagCheckResult, len(ps.checks))
	copy(out, ps.checks)
	return out, ps.deviceInfo, ps.lastStage
}

func makeTimeoutDiag(t DiagnosticsTarget, start time.Time, budget time.Duration, ps *diagPartialState) CameraDiagnostic {
	elapsed := time.Since(start).Milliseconds()
	checks, di, lastStage := ps.snapshot()
	msg := fmt.Sprintf("exceeded %s budget", budget)
	if lastStage != "" {
		msg += fmt.Sprintf("; timed out during %s", lastStage)
	}
	checks = append(checks, DiagCheckResult{
		Name:       "timeout",
		Status:     CheckFail,
		Message:    msg,
		DurationMS: elapsed,
	})
	errMsg := fmt.Sprintf("camera %s %s", t.Name, msg)
	return CameraDiagnostic{
		Name:       t.Name,
		IP:         t.IP,
		Port:       t.Port,
		Status:     StatusFail,
		DurationMS: elapsed,
		Checks:     checks,
		DeviceInfo: di,
		Errors:     []string{errMsg},
	}
}

func makeCancelledDiag(t DiagnosticsTarget, start time.Time) CameraDiagnostic {
	elapsed := time.Since(start).Milliseconds()
	return CameraDiagnostic{
		Name:       t.Name,
		IP:         t.IP,
		Port:       t.Port,
		Status:     StatusFail,
		DurationMS: elapsed,
		Checks: []DiagCheckResult{{
			Name:       "cancelled",
			Status:     CheckFail,
			Message:    "context cancelled",
			DurationMS: elapsed,
		}},
		Errors: []string{"cancelled"},
	}
}

// ComputeDiagStatus derives overall status from check results.
// Exported so tests and renderers can call it directly.
func ComputeDiagStatus(checks []DiagCheckResult) DiagnosticStatus {
	for _, c := range checks {
		if c.Status == CheckFail {
			return StatusFail
		}
	}
	for _, c := range checks {
		if c.Status == CheckWarn {
			return StatusWarn
		}
	}
	return StatusOK
}

// ── internal ──────────────────────────────────────────────────────────────────

// diagCamera runs all checks for one camera. partial receives each completed
// check result so the caller can build a meaningful result if a timeout fires
// before diagCamera returns.
func diagCamera(ctx context.Context, t DiagnosticsTarget, opts DiagnosticsOptions, stage func(string), partial *diagPartialState) CameraDiagnostic {
	wallStart := time.Now()
	d := CameraDiagnostic{Name: t.Name, IP: t.IP, Port: t.Port}

	dbg := func(format string, args ...any) {
		if opts.Debug {
			fmt.Fprintf(os.Stderr, "[diagnose:%s] "+format+"\n", append([]any{t.Name}, args...)...)
		}
	}

	// addCheck records one completed check to both the local result and the
	// shared partial state so a timeout can preserve it.
	addCheck := func(name string, status CheckStatus, msg string, durMS int64) {
		cr := DiagCheckResult{Name: name, Status: status, Message: msg, DurationMS: durMS}
		d.Checks = append(d.Checks, cr)
		partial.addCheck(cr)
		dbg("check %s: %s (%dms) %s", name, status, durMS, msg)
	}

	// 1. TCP — ONVIF port
	stage("tcp")
	partial.setStage("tcp-onvif")
	dbg("starting tcp-onvif %s:%d timeout=%s", t.IP, t.Port, opts.Timeout)
	t0 := time.Now()
	if err := diagTCPDial(ctx, fmt.Sprintf("%s:%d", t.IP, t.Port), opts.Timeout); err != nil {
		addCheck("tcp-onvif", CheckFail, fmt.Sprintf("connect failed: %v", err), diagMS(t0))
		d.Errors = append(d.Errors, fmt.Sprintf("TCP connect to %s:%d failed: %v", t.IP, t.Port, err))
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	addCheck("tcp-onvif", CheckOK, "", diagMS(t0))

	// 2. TCP — RTSP port (optional)
	if opts.SkipRTSP {
		addCheck("tcp-rtsp", CheckSkipped, "--skip-rtsp", 0)
	} else {
		stage("tcp-rtsp")
		partial.setStage("tcp-rtsp")
		dbg("starting tcp-rtsp %s:%d", t.IP, opts.RTSPPort)
		t0 = time.Now()
		if err := diagTCPDial(ctx, fmt.Sprintf("%s:%d", t.IP, opts.RTSPPort), opts.Timeout); err != nil {
			addCheck("tcp-rtsp", CheckWarn, fmt.Sprintf("port %d: %v", opts.RTSPPort, err), diagMS(t0))
			d.Warnings = append(d.Warnings, fmt.Sprintf("RTSP port %d not reachable", opts.RTSPPort))
		} else {
			addCheck("tcp-rtsp", CheckOK, "", diagMS(t0))
		}
	}

	// 3. Auth + device info — uses the same New() path as every other command.
	// The ONVIF library (use-go/onvif) calls GetCapabilities internally during
	// NewDevice construction. Per-camera timeout is enforced by the goroutine
	// wrapper in RunDiagnostics, not via http.Client.Timeout.
	stage("auth")
	partial.setStage("auth")
	dbg("starting auth %s:%d user=%s camera-timeout=%s", t.IP, t.Port, opts.Username, opts.CameraTimeout)
	t0 = time.Now()
	client, err := New(t.IP, t.Port, opts.Username, opts.Password)
	if err != nil {
		addCheck("auth", CheckFail, err.Error(), diagMS(t0))
		d.Errors = append(d.Errors, err.Error())
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	dbg("New() completed in %dms, calling GetDeviceInfo", diagMS(t0))
	info, err := client.GetDeviceInfo(ctx)
	if err != nil {
		addCheck("auth", CheckFail, err.Error(), diagMS(t0))
		d.Errors = append(d.Errors, fmt.Sprintf("device info: %v", err))
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	addCheck("auth", CheckOK, "", diagMS(t0))
	di := &DeviceInfoResult{
		Manufacturer:    info.Manufacturer,
		Model:           info.Model,
		FirmwareVersion: info.FirmwareVersion,
		SerialNumber:    info.SerialNumber,
		HardwareID:      info.HardwareID,
	}
	d.DeviceInfo = di
	partial.setDeviceInfo(di)

	// 4. Capabilities
	stage("capabilities")
	partial.setStage("capabilities")
	dbg("starting capabilities")
	t0 = time.Now()
	caps, err := client.GetCapabilities(ctx)
	if err != nil {
		addCheck("capabilities", CheckWarn, err.Error(), diagMS(t0))
		d.Warnings = append(d.Warnings, fmt.Sprintf("capabilities: %v", err))
	} else {
		addCheck("capabilities", CheckOK, "", diagMS(t0))
		d.Capabilities = &CapabilitiesResult{
			DeviceURL:  caps.DeviceXAddr,
			MediaURL:   caps.MediaXAddr,
			ImagingURL: caps.ImagingXAddr,
			EventsURL:  caps.EventsXAddr,
			PTZURL:     caps.PTZXAddr,
		}
	}

	// 5. Media profiles
	stage("profiles")
	partial.setStage("profiles")
	dbg("starting profiles")
	t0 = time.Now()
	profiles, err := client.GetProfiles(ctx)
	if err != nil {
		addCheck("profiles", CheckFail, err.Error(), diagMS(t0))
		d.Errors = append(d.Errors, fmt.Sprintf("profiles: %v", err))
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	if len(profiles) == 0 {
		addCheck("profiles", CheckFail, "no media profiles returned", diagMS(t0))
		d.Errors = append(d.Errors, "no media profiles returned")
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	var tokens, names []string
	for _, p := range profiles {
		tokens = append(tokens, p.Token)
		names = append(names, p.Name)
	}
	addCheck("profiles", CheckOK, fmt.Sprintf("%d profile(s)", len(profiles)), diagMS(t0))
	d.Profiles = &ProfilesResult{Count: len(profiles), Tokens: tokens, Names: names}

	// 6. Stream URI
	stage("stream-uri")
	partial.setStage("stream-uri")
	dbg("starting stream-uri")
	t0 = time.Now()
	streamTok, streamName, streamURI, err := client.GetStreamURI(ctx, "", StreamURIOptions{Transport: "rtsp"})
	streamDur := diagMS(t0)
	if err != nil {
		addCheck("stream-uri", CheckFail, err.Error(), streamDur)
		d.Errors = append(d.Errors, fmt.Sprintf("stream URI: %v", err))
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	if streamURI == "" {
		addCheck("stream-uri", CheckFail, "empty URI", streamDur)
		d.Errors = append(d.Errors, "stream URI: empty")
		d.Status = StatusFail
		d.DurationMS = diagMS(wallStart)
		return d
	}
	addCheck("stream-uri", CheckOK, "", streamDur)
	d.StreamURI = &StreamURIResult{
		ProfileToken: streamTok,
		ProfileName:  streamName,
		URI:          opts.RedactURI(streamURI),
	}

	// 7. Snapshot URI (optional)
	if opts.SkipSnapshot {
		addCheck("snapshot-uri", CheckSkipped, "--skip-snapshot", 0)
	} else {
		stage("snapshot-uri")
		partial.setStage("snapshot-uri")
		dbg("starting snapshot-uri")
		t0 = time.Now()
		snapTok, snapName, snapURI, err := client.GetSnapshotURI(ctx, "")
		snapDur := diagMS(t0)
		if err != nil {
			addCheck("snapshot-uri", CheckWarn, err.Error(), snapDur)
			d.Warnings = append(d.Warnings, fmt.Sprintf("snapshot URI: %v", err))
		} else {
			addCheck("snapshot-uri", CheckOK, "", snapDur)
			d.SnapshotURI = &SnapshotURIResult{
				ProfileToken: snapTok,
				ProfileName:  snapName,
				URI:          opts.RedactURI(snapURI),
			}
		}
	}

	// 8. Imaging (optional)
	if opts.SkipImaging {
		addCheck("imaging", CheckSkipped, "--skip-imaging", 0)
	} else {
		stage("imaging")
		partial.setStage("imaging")
		dbg("starting imaging")
		t0 = time.Now()
		vsToken, vsErr := client.GetVideoSourceToken(ctx)
		if vsErr != nil {
			addCheck("imaging", CheckWarn, fmt.Sprintf("video source: %v", vsErr), diagMS(t0))
			d.Warnings = append(d.Warnings, fmt.Sprintf("imaging video source: %v", vsErr))
		} else {
			settings, err := client.GetImagingSettings(ctx, vsToken)
			if err != nil {
				addCheck("imaging", CheckWarn, err.Error(), diagMS(t0))
				d.Warnings = append(d.Warnings, fmt.Sprintf("imaging settings: %v", err))
			} else {
				msg := ""
				if settings.Brightness == 0 && settings.Contrast == 0 &&
					settings.Saturation == 0 && settings.Sharpness == 0 {
					msg = "all zero values; may be normal for this model"
				}
				addCheck("imaging", CheckOK, msg, diagMS(t0))
				d.Imaging = &ImagingDiagResult{
					Brightness: settings.Brightness,
					Contrast:   settings.Contrast,
					Saturation: settings.Saturation,
					Sharpness:  settings.Sharpness,
				}
			}
		}
	}

	// 9. Network (optional)
	if opts.SkipNetwork {
		addCheck("network", CheckSkipped, "--skip-network", 0)
	} else {
		stage("network")
		partial.setStage("network")
		dbg("starting network")
		t0 = time.Now()
		ns := client.GetNetworkSummary(ctx)
		netDur := diagMS(t0)
		if len(ns.Warnings) > 0 {
			addCheck("network", CheckWarn, strings.Join(ns.Warnings, "; "), netDur)
			for _, w := range ns.Warnings {
				d.Warnings = append(d.Warnings, "network: "+w)
			}
		} else {
			addCheck("network", CheckOK, "", netDur)
		}
		if nr := buildNetworkDiagResult(ns); nr != nil {
			d.Network = nr
		}
	}

	// 10. WS-Discovery (always skipped until implemented)
	if opts.SkipDiscovery {
		addCheck("discovery", CheckSkipped, "pass --skip-discovery=false to enable (not yet implemented)", 0)
	}

	d.Status = ComputeDiagStatus(d.Checks)
	d.DurationMS = diagMS(wallStart)
	dbg("done: status=%s elapsed=%dms", d.Status, d.DurationMS)
	return d
}

func (d *CameraDiagnostic) addCheck(name string, status CheckStatus, msg string, durMS int64) {
	d.Checks = append(d.Checks, DiagCheckResult{
		Name:       name,
		Status:     status,
		Message:    msg,
		DurationMS: durMS,
	})
}

func diagTCPDial(ctx context.Context, addr string, timeout time.Duration) error {
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func diagMS(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

func buildNetworkDiagResult(ns NetworkSummary) *NetworkDiagResult {
	nr := &NetworkDiagResult{}
	hasData := false

	if ns.Hostname != nil {
		nr.Hostname = ns.Hostname.Name
		hasData = true
	}
	if len(ns.Interfaces) > 0 {
		iface := ns.Interfaces[0]
		nr.IfaceToken = iface.Token
		nr.IfaceName = iface.Name
		nr.MAC = iface.HwAddress
		nr.IPv4DHCP = iface.IPv4DHCP
		if iface.IPv4FromDHCP != nil {
			nr.IPv4Address = iface.IPv4FromDHCP.String()
		} else if len(iface.IPv4Manual) > 0 {
			nr.IPv4Address = iface.IPv4Manual[0].String()
		}
		hasData = true
	}
	if len(ns.Gateway) > 0 {
		nr.Gateway = strings.Join(ns.Gateway, ", ")
		hasData = true
	}
	if ns.DNS != nil {
		nr.DNSFromDHCP = ns.DNS.FromDHCP
		if len(ns.DNS.ManualAddrs) > 0 {
			nr.DNSServers = strings.Join(ns.DNS.ManualAddrs, ", ")
		}
		hasData = true
	}
	if ns.NTP != nil {
		nr.NTPFromDHCP = ns.NTP.FromDHCP
		if len(ns.NTP.Manual) > 0 {
			nr.NTPServers = strings.Join(ns.NTP.Manual, ", ")
		}
		hasData = true
	}

	if !hasData {
		return nil
	}
	return nr
}
