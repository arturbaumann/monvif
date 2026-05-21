package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arturbaumann/monvif/internal/camera"
)

// ---------------------------------------------------------------------------
// formatFPS
// ---------------------------------------------------------------------------

func TestFormatFPS(t *testing.T) {
	cases := []struct {
		fps  float64
		want string
	}{
		{30, "30"},
		{25, "25"},
		{15, "15"},
		{5, "5"},
		// Exact output of ParseFraction("30000/1001") ≈ 29.97
		{29.97002997002997, "29.97"},
		{29.97, "29.97"},
		{23.976, "23.98"},
		{23.976023976023978, "23.98"},
	}
	for _, tc := range cases {
		got := formatFPS(tc.fps)
		if got != tc.want {
			t.Errorf("formatFPS(%v) = %q, want %q", tc.fps, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// formatBitrate — compact "k"/"M"/"b" format
// ---------------------------------------------------------------------------

func TestFormatBitrate(t *testing.T) {
	cases := []struct {
		bps  int64
		want string
	}{
		{500, "500b"},
		{1000, "1k"},
		{1500, "2k"}, // 1.5 rounds to 2
		{512_000, "512k"},
		{800_000, "800k"},
		{999_999, "1000k"},
		{1_000_000, "1.0M"},
		{2_500_000, "2.5M"},
		{10_000_000, "10.0M"},
		// requirement 1: 4 * 1024 * 1024 = 4194304 bps
		{4_194_304, "4.2M"},
	}
	for _, tc := range cases {
		got := formatBitrate(tc.bps)
		if got != tc.want {
			t.Errorf("formatBitrate(%d) = %q, want %q", tc.bps, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// rowToJSON — flat JSON struct construction
// ---------------------------------------------------------------------------

func boolPtr(b bool) *bool { return &b }

func TestRowToJSON_NoProbe(t *testing.T) {
	r := streamProfileRow{
		Token:     "tok1",
		Name:      "Main",
		StreamURI: "rtsp://***@host/s",
	}
	j := rowToJSON(r)
	if j.Token != "tok1" || j.Name != "Main" || j.StreamURI != "rtsp://***@host/s" {
		t.Errorf("unexpected base fields: %+v", j)
	}
	if j.Works != nil {
		t.Error("Works must be nil when no probe was done")
	}
	if j.Codec != "" || j.Resolution != "" || j.Error != "" {
		t.Error("probe fields must be zero when no probe was done")
	}
}

func TestRowToJSON_ProbeSuccess(t *testing.T) {
	probe := camera.StreamProbeResult{
		Codec:      "h264",
		Width:      2560,
		Height:     1440,
		FPS:        20,
		BitrateBps: 4_194_304,
	}
	r := streamProfileRow{
		Token:     "tok1",
		Name:      "Main",
		StreamURI: "rtsp://***@host/s",
		Probe:     &probe,
	}
	j := rowToJSON(r)
	if j.Works == nil || !*j.Works {
		t.Error("Works must be true on successful probe")
	}
	if j.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", j.Codec)
	}
	if j.Width != 2560 || j.Height != 1440 {
		t.Errorf("dimensions = %dx%d, want 2560x1440", j.Width, j.Height)
	}
	if j.Resolution != "2560x1440" {
		t.Errorf("Resolution = %q, want 2560x1440", j.Resolution)
	}
	if j.FPS != 20 {
		t.Errorf("FPS = %v, want 20", j.FPS)
	}
	if j.Bitrate != "4.2M" {
		t.Errorf("Bitrate = %q, want 4.2M", j.Bitrate)
	}
	if j.Error != "" {
		t.Errorf("Error = %q, want empty on success", j.Error)
	}
}

func TestRowToJSON_ProbeError(t *testing.T) {
	probe := camera.StreamProbeResult{
		Error: "ffprobe not found in $PATH: install ffprobe (https://ffmpeg.org/download)",
	}
	r := streamProfileRow{
		Token: "tok2",
		Name:  "Sub",
		Probe: &probe,
	}
	j := rowToJSON(r)
	if j.Works == nil || *j.Works {
		t.Error("Works must be false when probe has an error")
	}
	if j.Error == "" {
		t.Error("Error must be non-empty when probe failed")
	}
	if j.Codec != "" || j.Resolution != "" {
		t.Error("codec/resolution must be absent on failed probe")
	}
}

func TestRowToJSON_NoBitrate(t *testing.T) {
	probe := camera.StreamProbeResult{
		Codec:  "h264",
		Width:  640,
		Height: 480,
		FPS:    15,
		// BitrateBps = 0: ffprobe didn't report it
	}
	r := streamProfileRow{Token: "t", Name: "n", Probe: &probe}
	j := rowToJSON(r)
	if j.Bitrate != "-" {
		t.Errorf("Bitrate must be \"-\" when BitrateBps=0, got %q", j.Bitrate)
	}
}

func TestRowToJSON_URIAlwaysRedacted(t *testing.T) {
	// The StreamURI stored in the row is already redacted by the caller.
	// Verify rowToJSON passes it through unchanged.
	r := streamProfileRow{
		Token:     "t",
		Name:      "n",
		StreamURI: "rtsp://***@192.0.2.1:554/1/1",
	}
	j := rowToJSON(r)
	if j.StreamURI != r.StreamURI {
		t.Errorf("StreamURI = %q, want %q", j.StreamURI, r.StreamURI)
	}
}

// TestRowToJSON_VerifiedCameraOutput checks the exact profile data returned by
// the verified real camera (h264 2560x1440 25fps, no bitrate reported).
func TestRowToJSON_VerifiedCameraOutput(t *testing.T) {
	probe := camera.StreamProbeResult{
		Codec:  "h264",
		Width:  2560,
		Height: 1440,
		FPS:    25,
		// BitrateBps intentionally 0 — camera does not report bitrate
	}
	r := streamProfileRow{
		Token:     "protoken_ch0001",
		Name:      "proname_ch0001",
		StreamURI: "rtsp://192.0.2.1:554/1/1",
		Probe:     &probe,
	}
	j := rowToJSON(r)
	if j.Works == nil || !*j.Works {
		t.Error("Works must be true when probe succeeds even without bitrate")
	}
	if j.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", j.Codec)
	}
	if j.Resolution != "2560x1440" {
		t.Errorf("Resolution = %q, want 2560x1440", j.Resolution)
	}
	if j.FPS != 25 {
		t.Errorf("FPS = %v, want 25", j.FPS)
	}
	if j.Bitrate != "-" {
		t.Errorf("Bitrate must be \"-\" when BitrateBps=0, got %q", j.Bitrate)
	}
	if j.Error != "" {
		t.Errorf("Error must be empty on success: %q", j.Error)
	}
}

// TestRowToJSON_StableSchema verifies that bitrate and error are always present
// in JSON output regardless of probe status, so automation scripts can rely on
// a fixed schema without defensive nil-checks.
func TestRowToJSON_StableSchema_NoProbe(t *testing.T) {
	j := rowToJSON(streamProfileRow{Token: "t", Name: "n", StreamURI: "rtsp://***@host/1"})
	// bitrate must be "-", not omitted
	if j.Bitrate != "-" {
		t.Errorf("Bitrate = %q, want \"-\" when probe not run", j.Bitrate)
	}
	// error must be "", not omitted
	if j.Error != "" {
		t.Errorf("Error = %q, want \"\" when probe not run", j.Error)
	}
}

func TestRowToJSON_StableSchema_ProbeSuccess(t *testing.T) {
	j := rowToJSON(streamProfileRow{
		Token: "t", Name: "n",
		Probe: &camera.StreamProbeResult{Codec: "h264", Width: 1920, Height: 1080, FPS: 30, BitrateBps: 2_000_000},
	})
	if j.Bitrate != "2.0M" {
		t.Errorf("Bitrate = %q, want \"2.0M\"", j.Bitrate)
	}
	if j.Error != "" {
		t.Errorf("Error = %q, want \"\" on success", j.Error)
	}
}

func TestRowToJSON_StableSchema_ProbeSuccessNoBitrate(t *testing.T) {
	j := rowToJSON(streamProfileRow{
		Token: "t", Name: "n",
		Probe: &camera.StreamProbeResult{Codec: "h264", Width: 1920, Height: 1080, FPS: 30},
	})
	if j.Bitrate != "-" {
		t.Errorf("Bitrate = %q, want \"-\" when ffprobe reports no bitrate", j.Bitrate)
	}
	if j.Error != "" {
		t.Errorf("Error = %q, want \"\" on success", j.Error)
	}
}

func TestRowToJSON_StableSchema_ProbeFailure(t *testing.T) {
	j := rowToJSON(streamProfileRow{
		Token: "t", Name: "n",
		Probe: &camera.StreamProbeResult{Error: "RTSP auth failed"},
	})
	if j.Bitrate != "-" {
		t.Errorf("Bitrate = %q, want \"-\" on probe failure", j.Bitrate)
	}
	if j.Error != "RTSP auth failed" {
		t.Errorf("Error = %q, want \"RTSP auth failed\"", j.Error)
	}
}

// ---------------------------------------------------------------------------
// renderStreamProfilesTable
// ---------------------------------------------------------------------------

func TestRenderStreamProfilesTable_NoProbeMode(t *testing.T) {
	rows := []streamProfileRow{
		{Token: "tok1", Name: "Main", StreamURI: "rtsp://***@host/1"},
		{Token: "tok2", Name: "Sub", StreamURI: "rtsp://***@host/2"},
	}
	var buf bytes.Buffer
	if err := renderStreamProfilesTable(&buf, rows, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "tok1") || !strings.Contains(out, "rtsp://***@host/1") {
		t.Errorf("table missing expected row data: %q", out)
	}
	if strings.Contains(out, "CODEC") {
		t.Error("no-probe table must not contain CODEC column")
	}
	if strings.Contains(out, "WORKS") {
		t.Error("no-probe table must not contain WORKS column")
	}
}

func TestRenderStreamProfilesTable_ProbeNoBitrate(t *testing.T) {
	// Verified camera output: h264, no bitrate reported → WORKS=yes, BITRATE=-.
	rows := []streamProfileRow{
		{Token: "tok1", Name: "Main", Probe: &camera.StreamProbeResult{
			Codec: "h264", Width: 2560, Height: 1440, FPS: 25,
		}},
	}
	var buf bytes.Buffer
	if err := renderStreamProfilesTable(&buf, rows, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "yes") {
		t.Errorf("expected WORKS=yes: %q", out)
	}
	if strings.Contains(out, "ERROR") {
		t.Error("ERROR column must not appear when all probes succeed")
	}
}

func TestRenderStreamProfilesTable_PartialProbeFailure(t *testing.T) {
	// One profile succeeds, one fails — ERROR column must appear; command must not abort.
	rows := []streamProfileRow{
		{Token: "tok1", Name: "Main", Probe: &camera.StreamProbeResult{
			Codec: "h264", Width: 1920, Height: 1080, FPS: 30,
		}},
		{Token: "tok2", Name: "Sub", Probe: &camera.StreamProbeResult{
			Error: "RTSP auth failed",
		}},
	}
	var buf bytes.Buffer
	if err := renderStreamProfilesTable(&buf, rows, true); err != nil {
		t.Fatalf("partial failure must not abort table rendering: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ERROR") {
		t.Error("ERROR column must appear when at least one probe fails")
	}
	if !strings.Contains(out, "RTSP auth failed") {
		t.Errorf("expected error message in output: %q", out)
	}
	if !strings.Contains(out, "yes") {
		t.Errorf("successful row must show WORKS=yes: %q", out)
	}
	if !strings.Contains(out, "no") {
		t.Errorf("failed row must show WORKS=no: %q", out)
	}
	// Both rows must appear.
	if !strings.Contains(out, "tok1") || !strings.Contains(out, "tok2") {
		t.Errorf("both rows must be present: %q", out)
	}
}
