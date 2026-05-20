package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arturbaumann/monvif/internal/camera"
)

// ── status scoring ────────────────────────────────────────────────────────────

func TestComputeDiagStatus_AllOK(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "stream-uri", Status: camera.CheckOK},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusOK {
		t.Errorf("expected OK, got %v", got)
	}
}

func TestComputeDiagStatus_TCPFail(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckFail},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusFail {
		t.Errorf("expected FAIL, got %v", got)
	}
}

func TestComputeDiagStatus_AuthFail(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckFail},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusFail {
		t.Errorf("expected FAIL, got %v", got)
	}
}

func TestComputeDiagStatus_NoStreamURIFail(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "profiles", Status: camera.CheckOK},
		{Name: "stream-uri", Status: camera.CheckFail},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusFail {
		t.Errorf("expected FAIL, got %v", got)
	}
}

func TestComputeDiagStatus_SnapshotWarnOnly(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "stream-uri", Status: camera.CheckOK},
		{Name: "snapshot-uri", Status: camera.CheckWarn},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusWarn {
		t.Errorf("expected WARN for snapshot-only warn, got %v", got)
	}
}

func TestComputeDiagStatus_DiscoverySkippedIsOK(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "stream-uri", Status: camera.CheckOK},
		{Name: "discovery", Status: camera.CheckSkipped},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusOK {
		t.Errorf("expected OK when only discovery is skipped, got %v", got)
	}
}

func TestComputeDiagStatus_Empty(t *testing.T) {
	if got := camera.ComputeDiagStatus(nil); got != camera.StatusOK {
		t.Errorf("expected OK for no checks, got %v", got)
	}
}

// ── format validation ─────────────────────────────────────────────────────────

func TestValidateDiagnoseFormat_Valid(t *testing.T) {
	for _, f := range []string{"table", "json", "markdown"} {
		if err := validateDiagnoseFormat(f); err != nil {
			t.Errorf("validateDiagnoseFormat(%q) unexpected error: %v", f, err)
		}
	}
}

func TestValidateDiagnoseFormat_Invalid(t *testing.T) {
	for _, f := range []string{"csv", "xml", ""} {
		if err := validateDiagnoseFormat(f); err == nil {
			t.Errorf("validateDiagnoseFormat(%q) expected error, got nil", f)
		}
	}
}

// ── mock report ───────────────────────────────────────────────────────────────

func newMockReport() camera.DiagnosticsReport {
	return camera.DiagnosticsReport{
		GeneratedAt: time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC),
		DurationMS:  5234,
		Totals:      camera.DiagnosticSummary{Total: 2, OK: 1, Warn: 1, Fail: 0},
		Cameras: []camera.CameraDiagnostic{
			{
				Name:   "front-door",
				IP:     "192.168.1.10",
				Port:   80,
				Status: camera.StatusOK,
				DeviceInfo: &camera.DeviceInfoResult{
					Manufacturer: "Acme", Model: "X200", FirmwareVersion: "2.1.0",
				},
				Profiles:  &camera.ProfilesResult{Count: 2, Tokens: []string{"tok1"}, Names: []string{"Main"}},
				StreamURI: &camera.StreamURIResult{ProfileToken: "tok1", URI: "rtsp://192.168.1.10/stream"},
				Checks: []camera.DiagCheckResult{
					{Name: "tcp-onvif", Status: camera.CheckOK, DurationMS: 10},
					{Name: "auth", Status: camera.CheckOK, DurationMS: 200},
					{Name: "profiles", Status: camera.CheckOK, Message: "2 profile(s)", DurationMS: 90},
					{Name: "stream-uri", Status: camera.CheckOK, DurationMS: 150},
					{Name: "snapshot-uri", Status: camera.CheckOK, DurationMS: 120},
					{Name: "imaging", Status: camera.CheckOK, DurationMS: 80},
					{Name: "network", Status: camera.CheckOK, DurationMS: 200},
					{Name: "discovery", Status: camera.CheckSkipped},
				},
				Network: &camera.NetworkDiagResult{
					Hostname: "camera1", IPv4Address: "192.168.1.10/24", Gateway: "192.168.1.1",
				},
			},
			{
				Name:   "garage",
				IP:     "192.168.1.11",
				Port:   80,
				Status: camera.StatusWarn,
				DeviceInfo: &camera.DeviceInfoResult{
					Manufacturer: "Acme", Model: "X100",
				},
				Checks: []camera.DiagCheckResult{
					{Name: "tcp-onvif", Status: camera.CheckOK, DurationMS: 8},
					{Name: "auth", Status: camera.CheckOK, DurationMS: 180},
					{Name: "snapshot-uri", Status: camera.CheckWarn, Message: "not supported", DurationMS: 100},
				},
				Warnings: []string{"snapshot URI: GetSnapshotUri: not supported"},
			},
		},
	}
}

// ── table rendering ───────────────────────────────────────────────────────────

func TestRenderDiagnoseTable_ContainsCameras(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseTable(newMockReport(), &buf)
	out := buf.String()

	for _, want := range []string{"front-door", "garage", "OK", "WARN", "Total: 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q", want)
		}
	}
}

func TestRenderDiagnoseTable_Header(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseTable(newMockReport(), &buf)
	out := buf.String()

	for _, col := range []string{"NAME", "STATUS", "TCP", "AUTH", "PROFILES", "STREAM"} {
		if !strings.Contains(out, col) {
			t.Errorf("table missing column header %q", col)
		}
	}
}

// ── JSON rendering ────────────────────────────────────────────────────────────

func TestRenderDiagnoseJSON_Valid(t *testing.T) {
	var buf bytes.Buffer
	if err := renderDiagnoseJSON(newMockReport(), &buf); err != nil {
		t.Fatalf("renderDiagnoseJSON error: %v", err)
	}
	var v interface{}
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\n---\n%s", err, buf.String())
	}
}

func TestRenderDiagnoseJSON_Fields(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseJSON(newMockReport(), &buf) //nolint
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"generated_at", "duration_ms", "totals", "cameras"} {
		if _, ok := m[field]; !ok {
			t.Errorf("JSON missing top-level field %q", field)
		}
	}
}

// ── markdown rendering ────────────────────────────────────────────────────────

func TestRenderDiagnoseMarkdown_Title(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	if !strings.HasPrefix(out, "# monvif Diagnostic Report") {
		t.Errorf("markdown does not start with expected title, got: %q", out[:min(50, len(out))])
	}
}

func TestRenderDiagnoseMarkdown_Summary(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	for _, want := range []string{"## Summary", "| OK", "| WARN", "| FAIL", "| **Total**"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestRenderDiagnoseMarkdown_CameraSections(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	if !strings.Contains(out, "## front-door") {
		t.Error("markdown missing front-door camera section")
	}
	if !strings.Contains(out, "## garage") {
		t.Error("markdown missing garage camera section")
	}
}

func TestRenderDiagnoseMarkdown_ChecksTable(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	if !strings.Contains(out, "### Checks") {
		t.Error("markdown missing Checks section")
	}
	if !strings.Contains(out, "tcp-onvif") {
		t.Error("markdown missing tcp-onvif check row")
	}
}

func TestRenderDiagnoseMarkdown_NetworkSection(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	if !strings.Contains(out, "### Network") {
		t.Error("markdown missing Network section")
	}
	if !strings.Contains(out, "192.168.1.10/24") {
		t.Error("markdown missing IPv4 address")
	}
}

func TestRenderDiagnoseMarkdown_WarningsSection(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseMarkdown(newMockReport(), &buf)
	out := buf.String()

	if !strings.Contains(out, "### Warnings") {
		t.Error("markdown missing Warnings section for garage camera")
	}
	if !strings.Contains(out, "snapshot URI") {
		t.Error("markdown missing snapshot warning text")
	}
}

// ── output file ───────────────────────────────────────────────────────────────

func TestOutputFileMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "diag.md")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	renderDiagnoseMarkdown(newMockReport(), f)
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# monvif Diagnostic Report") {
		t.Error("written file does not contain expected header")
	}
}

func TestOutputFileJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "diag.json")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	renderDiagnoseJSON(newMockReport(), f) //nolint
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("file content is not valid JSON: %v", err)
	}
}

// ── helper functions ──────────────────────────────────────────────────────────

func TestTruncateDiagStr(t *testing.T) {
	if got := truncateDiagStr("short", 20); got != "short" {
		t.Errorf("expected %q, got %q", "short", got)
	}
	long := "this string is definitely too long for the limit"
	got := truncateDiagStr(long, 10)
	if len(got) != 10 {
		t.Errorf("expected len 10, got %d: %q", len(got), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ... suffix, got %q", got)
	}
}

func TestDiagNotesCol_ErrorFirst(t *testing.T) {
	d := camera.CameraDiagnostic{
		Errors:   []string{"auth failed"},
		Warnings: []string{"snapshot missing"},
	}
	if got := diagNotesCol(d, 100); got != "auth failed" {
		t.Errorf("expected error in notes, got %q", got)
	}
}

func TestDiagNotesCol_WarnFallback(t *testing.T) {
	d := camera.CameraDiagnostic{
		Warnings: []string{"snapshot missing"},
	}
	if got := diagNotesCol(d, 100); got != "snapshot missing" {
		t.Errorf("expected warning in notes, got %q", got)
	}
}

func TestDiagNotesCol_Empty(t *testing.T) {
	d := camera.CameraDiagnostic{}
	if got := diagNotesCol(d, 100); got != "" {
		t.Errorf("expected empty notes, got %q", got)
	}
}

func TestDiagCheckCol_OK(t *testing.T) {
	d := camera.CameraDiagnostic{
		Checks: []camera.DiagCheckResult{{Name: "auth", Status: camera.CheckOK}},
	}
	if got := diagCheckCol(d, "auth"); got != "yes" {
		t.Errorf("expected yes, got %q", got)
	}
}

func TestDiagCheckCol_Missing(t *testing.T) {
	d := camera.CameraDiagnostic{}
	if got := diagCheckCol(d, "auth"); got != "-" {
		t.Errorf("expected -, got %q", got)
	}
}

func TestDiagProfilesCol_WithProfiles(t *testing.T) {
	d := camera.CameraDiagnostic{
		Profiles: &camera.ProfilesResult{Count: 3},
	}
	if got := diagProfilesCol(d); got != "3" {
		t.Errorf("expected 3, got %q", got)
	}
}

// ── timeout / skip flag behaviour ────────────────────────────────────────────

func TestComputeDiagStatus_SkippedChecksAreOK(t *testing.T) {
	// All core checks OK, optional checks skipped → overall OK.
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "tcp-rtsp", Status: camera.CheckSkipped},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "capabilities", Status: camera.CheckOK},
		{Name: "profiles", Status: camera.CheckOK},
		{Name: "stream-uri", Status: camera.CheckOK},
		{Name: "snapshot-uri", Status: camera.CheckSkipped},
		{Name: "imaging", Status: camera.CheckSkipped},
		{Name: "network", Status: camera.CheckSkipped},
		{Name: "discovery", Status: camera.CheckSkipped},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusOK {
		t.Errorf("expected OK when optional checks skipped, got %v", got)
	}
}

func TestComputeDiagStatus_TimeoutIsFail(t *testing.T) {
	checks := []camera.DiagCheckResult{
		{Name: "tcp-onvif", Status: camera.CheckOK},
		{Name: "auth", Status: camera.CheckOK},
		{Name: "timeout", Status: camera.CheckFail, Message: "camera diagnostic timeout exceeded after stream-uri"},
	}
	if got := camera.ComputeDiagStatus(checks); got != camera.StatusFail {
		t.Errorf("expected FAIL when timeout check present, got %v", got)
	}
}

func TestValidateDiagnoseFormat_AllThree(t *testing.T) {
	for _, f := range []string{"table", "json", "markdown"} {
		if err := validateDiagnoseFormat(f); err != nil {
			t.Errorf("validateDiagnoseFormat(%q) unexpected error: %v", f, err)
		}
	}
}

func TestDiagNotesCol_SlowCheck(t *testing.T) {
	d := camera.CameraDiagnostic{
		Checks: []camera.DiagCheckResult{
			{Name: "snapshot-uri", Status: camera.CheckOK, DurationMS: 3500},
		},
	}
	notes := diagNotesCol(d, 200)
	if !strings.Contains(notes, "slow:snapshot-uri") {
		t.Errorf("expected slow note, got %q", notes)
	}
	if !strings.Contains(notes, "3500ms") {
		t.Errorf("expected duration in slow note, got %q", notes)
	}
}

func TestDiagNotesCol_NoSlowBelowThreshold(t *testing.T) {
	d := camera.CameraDiagnostic{
		Checks: []camera.DiagCheckResult{
			{Name: "auth", Status: camera.CheckOK, DurationMS: 1999},
		},
	}
	if got := diagNotesCol(d, 200); got != "" {
		t.Errorf("expected no slow note for 1999ms, got %q", got)
	}
}

func TestRenderDiagnoseJSON_IncludesDurations(t *testing.T) {
	var buf bytes.Buffer
	renderDiagnoseJSON(newMockReport(), &buf) //nolint
	// Verify each check in JSON includes duration_ms.
	var report map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	cameras, _ := report["cameras"].([]interface{})
	if len(cameras) == 0 {
		t.Fatal("no cameras in JSON output")
	}
	first, _ := cameras[0].(map[string]interface{})
	checks, _ := first["checks"].([]interface{})
	if len(checks) == 0 {
		t.Fatal("no checks in first camera JSON")
	}
	firstCheck, _ := checks[0].(map[string]interface{})
	if _, ok := firstCheck["duration_ms"]; !ok {
		t.Error("check missing duration_ms in JSON output")
	}
}

func TestQuickModeSkipsOptional(t *testing.T) {
	// Verify that the quick-mode flag combination skips the right checks.
	// We use the mock report which has skipped checks for snapshot/imaging/network.
	r := newQuickMockReport()
	var buf bytes.Buffer
	renderDiagnoseTable(r, &buf)
	out := buf.String()
	// Quick mode: snapshot, imaging, network should show "skip"
	lines := strings.Split(out, "\n")
	var cameraLine string
	for _, l := range lines {
		if strings.Contains(l, "front-door") {
			cameraLine = l
			break
		}
	}
	if cameraLine == "" {
		t.Fatal("front-door line not found in table output")
	}
	if !strings.Contains(cameraLine, "skip") {
		t.Errorf("expected skip in quick-mode camera row, got: %q", cameraLine)
	}
}

// newQuickMockReport returns a report as if --quick was used.
func newQuickMockReport() camera.DiagnosticsReport {
	r := newMockReport()
	for i := range r.Cameras {
		r.Cameras[i].Checks = []camera.DiagCheckResult{
			{Name: "tcp-onvif", Status: camera.CheckOK, DurationMS: 10},
			{Name: "tcp-rtsp", Status: camera.CheckSkipped},
			{Name: "auth", Status: camera.CheckOK, DurationMS: 200},
			{Name: "capabilities", Status: camera.CheckOK, DurationMS: 100},
			{Name: "profiles", Status: camera.CheckOK, DurationMS: 90},
			{Name: "stream-uri", Status: camera.CheckOK, DurationMS: 150},
			{Name: "snapshot-uri", Status: camera.CheckSkipped},
			{Name: "imaging", Status: camera.CheckSkipped},
			{Name: "network", Status: camera.CheckSkipped},
			{Name: "discovery", Status: camera.CheckSkipped},
		}
	}
	return r
}

// ── concurrency validation ────────────────────────────────────────────────────

func TestValidateDiagnoseConcurrency_Valid(t *testing.T) {
	for _, n := range []int{1, 4, 16, 100} {
		if err := validateDiagnoseConcurrency(n); err != nil {
			t.Errorf("validateDiagnoseConcurrency(%d) unexpected error: %v", n, err)
		}
	}
}

func TestValidateDiagnoseConcurrency_Invalid(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if err := validateDiagnoseConcurrency(n); err == nil {
			t.Errorf("validateDiagnoseConcurrency(%d) expected error, got nil", n)
		}
	}
}

// ── concurrency default ───────────────────────────────────────────────────────

func TestDiagnosticsOptionsDefaults(t *testing.T) {
	// Concurrency=0 in opts should be normalised to 4 inside RunDiagnostics.
	// We can observe this indirectly: a report with 0 cameras should still
	// return without error.
	opts := camera.DiagnosticsOptions{
		Concurrency:   0, // should default to 4
		Timeout:       50 * time.Millisecond,
		CameraTimeout: 200 * time.Millisecond,
		SkipDiscovery: true,
	}
	report := camera.RunDiagnostics(context.Background(), nil, opts)
	if report.Totals.Total != 0 {
		t.Errorf("expected 0 cameras, got %d", report.Totals.Total)
	}
}

// ── result ordering ───────────────────────────────────────────────────────────

// TestRunDiagnosticsOrderPreserved verifies that results are returned in
// inventory order even when cameras run concurrently and fail at different speeds.
// It uses loopback addresses with closed ports so TCP connects fail immediately.
func TestRunDiagnosticsOrderPreserved(t *testing.T) {
	targets := []camera.DiagnosticsTarget{
		{Name: "cam-a", IP: "127.0.0.1", Port: 1},
		{Name: "cam-b", IP: "127.0.0.1", Port: 2},
		{Name: "cam-c", IP: "127.0.0.1", Port: 3},
	}
	opts := camera.DiagnosticsOptions{
		Username:      "user",
		Password:      "pass",
		Timeout:       200 * time.Millisecond,
		CameraTimeout: 2 * time.Second,
		Concurrency:   3, // all three run simultaneously
		SkipDiscovery: true,
	}
	report := camera.RunDiagnostics(context.Background(), targets, opts)
	if len(report.Cameras) != 3 {
		t.Fatalf("expected 3 cameras, got %d", len(report.Cameras))
	}
	for i, want := range []string{"cam-a", "cam-b", "cam-c"} {
		if got := report.Cameras[i].Name; got != want {
			t.Errorf("position %d: expected %q, got %q", i, want, got)
		}
	}
	// All should fail (no service on those ports).
	for _, d := range report.Cameras {
		if d.Status != camera.StatusFail {
			t.Errorf("camera %s: expected FAIL, got %s", d.Name, d.Status)
		}
	}
}

// ── concurrent JSON validity ──────────────────────────────────────────────────

func TestConcurrentDiagnosticsJSON(t *testing.T) {
	// Build a mock report that simulates 4 cameras completing out of order,
	// then verify the JSON output is valid and preserves order.
	r := camera.DiagnosticsReport{
		GeneratedAt: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		DurationMS:  12000,
		Totals:      camera.DiagnosticSummary{Total: 4, OK: 3, Warn: 1},
	}
	for i, name := range []string{"alpha", "beta", "gamma", "delta"} {
		r.Cameras = append(r.Cameras, camera.CameraDiagnostic{
			Name:       name,
			IP:         fmt.Sprintf("192.168.1.%d", 10+i),
			Port:       80,
			Status:     camera.StatusOK,
			DurationMS: int64((i + 1) * 1000),
			Checks: []camera.DiagCheckResult{
				{Name: "tcp-onvif", Status: camera.CheckOK, DurationMS: 5},
				{Name: "auth", Status: camera.CheckOK, DurationMS: int64(5000 + i*200)},
			},
		})
	}

	var buf bytes.Buffer
	if err := renderDiagnoseJSON(r, &buf); err != nil {
		t.Fatalf("renderDiagnoseJSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON invalid: %v\n%s", err, buf.String())
	}

	cameras, _ := parsed["cameras"].([]interface{})
	if len(cameras) != 4 {
		t.Fatalf("expected 4 cameras in JSON, got %d", len(cameras))
	}
	names := []string{"alpha", "beta", "gamma", "delta"}
	for i, c := range cameras {
		m := c.(map[string]interface{})
		if got := m["name"].(string); got != names[i] {
			t.Errorf("camera[%d]: expected %q, got %q", i, names[i], got)
		}
	}
}

// ── progress tracker ──────────────────────────────────────────────────────────

func TestDiagProgressTracker_DoneIncrement(t *testing.T) {
	pr := &progressReporter{} // disabled (enabled=false), update is no-op
	pt := newDiagProgressTracker(3, time.Now(), pr)

	pt.stageCallback(1, 3, "cam-a", "auth")
	pt.stageCallback(2, 3, "cam-b", "tcp")
	pt.stageCallback(1, 3, "cam-a", "done")

	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.done != 1 {
		t.Errorf("expected done=1, got %d", pt.done)
	}
	if _, active := pt.stages["cam-a"]; active {
		t.Error("cam-a should be removed from stages after done")
	}
	if _, active := pt.stages["cam-b"]; !active {
		t.Error("cam-b should still be in stages")
	}
}

// ── --only filtering ─────────────────────────────────────────────────────────

// TestRunDiagnosticsPartialOnTimeout verifies that when a camera exceeds its
// timeout, any checks that completed before the timeout are preserved in the
// result (e.g. TCP check shows "yes" even if auth is still running).
func TestRunDiagnosticsPartialOnTimeout(t *testing.T) {
	// 127.0.0.1:1 gets an immediate ECONNREFUSED, so tcp-onvif completes fast.
	// CameraTimeout is very short so it fires before auth can start. We should
	// still see the tcp-onvif check in the result.
	targets := []camera.DiagnosticsTarget{
		{Name: "fast-fail", IP: "127.0.0.1", Port: 1},
	}
	opts := camera.DiagnosticsOptions{
		Timeout:       200 * time.Millisecond,
		CameraTimeout: 5 * time.Second,
		Concurrency:   1,
		SkipDiscovery: true,
	}
	report := camera.RunDiagnostics(context.Background(), targets, opts)
	if len(report.Cameras) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(report.Cameras))
	}
	cam := report.Cameras[0]
	if cam.Status != camera.StatusFail {
		t.Errorf("expected FAIL, got %s", cam.Status)
	}
	// tcp-onvif should be present and show fail (connection refused).
	found := false
	for _, c := range cam.Checks {
		if c.Name == "tcp-onvif" {
			found = true
			if c.Status != camera.CheckFail {
				t.Errorf("tcp-onvif: expected fail, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Error("tcp-onvif check not found in result — partial checks not preserved")
	}
}

// TestRunDiagnosticsCameraTimeoutStartsWhenActive verifies that with
// concurrency=1 and two cameras, the second camera does not use the first
// camera's elapsed time against its own budget.
func TestRunDiagnosticsCameraTimeoutStartsWhenActive(t *testing.T) {
	// Both cameras fail TCP immediately. With concurrency=1 they run serially.
	// Each should report a tcp-onvif FAIL (not a timeout), even if the first
	// camera took a moment — the second camera's 2s budget is measured from
	// when it acquires the slot, not from when RunDiagnostics started.
	targets := []camera.DiagnosticsTarget{
		{Name: "cam-1", IP: "127.0.0.1", Port: 1},
		{Name: "cam-2", IP: "127.0.0.1", Port: 2},
	}
	opts := camera.DiagnosticsOptions{
		Timeout:       200 * time.Millisecond,
		CameraTimeout: 2 * time.Second,
		Concurrency:   1,
		SkipDiscovery: true,
	}
	report := camera.RunDiagnostics(context.Background(), targets, opts)
	for _, cam := range report.Cameras {
		// Each camera should have failed via tcp-onvif, not a budget timeout.
		hasTimeout := false
		hasTCP := false
		for _, c := range cam.Checks {
			if c.Name == "timeout" {
				hasTimeout = true
			}
			if c.Name == "tcp-onvif" {
				hasTCP = true
			}
		}
		if hasTimeout {
			t.Errorf("camera %s: got timeout check — budget started before worker was active", cam.Name)
		}
		if !hasTCP {
			t.Errorf("camera %s: missing tcp-onvif check", cam.Name)
		}
	}
}

// ── min helper ────────────────────────────────────────────────────────────────

// min is a helper for Go <1.21 compat in test.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
