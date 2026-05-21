package camera

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseFraction
// ---------------------------------------------------------------------------

func TestParseFraction(t *testing.T) {
	cases := []struct {
		input string
		want  float64
		eps   float64 // tolerance; 0 means exact equality required
	}{
		{"", 0, 0},
		{"0/0", 0, 0},
		{"0/1", 0, 0},
		{"30/1", 30, 0},
		{"25/1", 25, 0},
		{"60/2", 30, 0},
		{"25", 25, 0},
		// NTSC: 30000/1001 ≈ 29.97 (req 5)
		{"30000/1001", 29.97, 0.001},
		// PAL: 25000/1000 = 25
		{"25000/1000", 25, 0},
		// 20 fps as used in requirement 1 JSON
		{"20/1", 20, 0},
		// 15 fps for r_frame_rate fallback case
		{"15/1", 15, 0},
	}
	for _, tc := range cases {
		got := ParseFraction(tc.input)
		if tc.eps == 0 {
			if got != tc.want {
				t.Errorf("ParseFraction(%q) = %v, want %v", tc.input, got, tc.want)
			}
		} else {
			diff := got - tc.want
			if diff < -tc.eps || diff > tc.eps {
				t.Errorf("ParseFraction(%q) = %v, want ~%v (±%v)", tc.input, got, tc.want, tc.eps)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 1: normal H.264 stream
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_H264_2560x1440(t *testing.T) {
	// Exact JSON from the requirement specification.
	input := `{
		"streams": [
			{
				"codec_name": "h264",
				"codec_type": "video",
				"width": 2560,
				"height": 1440,
				"avg_frame_rate": "20/1",
				"bit_rate": "4194304"
			}
		],
		"format": {
			"bit_rate": "4194304"
		}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", r.Codec)
	}
	if r.Width != 2560 || r.Height != 1440 {
		t.Errorf("resolution = %dx%d, want 2560x1440", r.Width, r.Height)
	}
	if r.FPS != 20 {
		t.Errorf("FPS = %v, want 20", r.FPS)
	}
	if r.BitrateBps != 4194304 {
		t.Errorf("BitrateBps = %d, want 4194304", r.BitrateBps)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
}

// requirement 1 (continued): 1920x1080 baseline already tested here too
func TestParseFfprobeOutput_VideoStream(t *testing.T) {
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"avg_frame_rate": "30/1",
				"r_frame_rate": "30/1"
			}
		],
		"format": {
			"bit_rate": "2000000"
		}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", r.Codec)
	}
	if r.Width != 1920 || r.Height != 1080 {
		t.Errorf("resolution = %dx%d, want 1920x1080", r.Width, r.Height)
	}
	if r.FPS != 30 {
		t.Errorf("FPS = %v, want 30", r.FPS)
	}
	if r.BitrateBps != 2000000 {
		t.Errorf("BitrateBps = %d, want 2000000", r.BitrateBps)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 2: bitrate comes from format, not stream
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_UseFormatBitrate(t *testing.T) {
	// Stream entry has no bit_rate field; format has it.
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h265",
				"width": 1280,
				"height": 720,
				"avg_frame_rate": "25/1"
			}
		],
		"format": {
			"bit_rate": "500000"
		}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.BitrateBps != 500000 {
		t.Errorf("BitrateBps = %d, want 500000 (from format.bit_rate)", r.BitrateBps)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 3: no bitrate fields at all
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_NoBitrate(t *testing.T) {
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 640,
				"height": 480,
				"avg_frame_rate": "15/1"
			}
		],
		"format": {}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.BitrateBps != 0 {
		t.Errorf("BitrateBps = %d, want 0 when neither stream nor format has bit_rate", r.BitrateBps)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 4: avg_frame_rate is "0/0", fall back to r_frame_rate
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_FallbackToRFrameRate(t *testing.T) {
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h265",
				"width": 3840,
				"height": 2160,
				"avg_frame_rate": "0/0",
				"r_frame_rate": "15/1"
			}
		],
		"format": {}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.FPS != 15 {
		t.Errorf("FPS = %v, want 15 (fallback to r_frame_rate when avg_frame_rate is 0/0)", r.FPS)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 5: fractional fps (30000/1001 ≈ 29.97)
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_FractionalFPS(t *testing.T) {
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"avg_frame_rate": "30000/1001"
			}
		],
		"format": {}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 30000/1001 ≈ 29.97; accept ±0.01
	if r.FPS < 29.96 || r.FPS > 29.98 {
		t.Errorf("FPS = %v, want ~29.97 for 30000/1001", r.FPS)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 6: no video stream
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_NoVideoStream_EmptyStreams(t *testing.T) {
	input := `{"streams": [], "format": {}}`
	_, err := ParseFfprobeOutput(strings.NewReader(input))
	if !errors.Is(err, ErrNoVideoStream) {
		t.Errorf("expected ErrNoVideoStream, got %v", err)
	}
}

func TestParseFfprobeOutput_NoVideoStream_AudioOnly(t *testing.T) {
	// Only audio streams — no video.
	input := `{
		"streams": [
			{"codec_type": "audio", "codec_name": "aac"}
		],
		"format": {}
	}`
	_, err := ParseFfprobeOutput(strings.NewReader(input))
	if !errors.Is(err, ErrNoVideoStream) {
		t.Errorf("expected ErrNoVideoStream for audio-only input, got %v", err)
	}
}

// Verify audio stream before video is skipped and video is still found.
func TestParseFfprobeOutput_SkipsAudioStream(t *testing.T) {
	input := `{
		"streams": [
			{
				"codec_type": "audio",
				"codec_name": "aac"
			},
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 640,
				"height": 480,
				"avg_frame_rate": "15/1",
				"r_frame_rate": "15/1"
			}
		],
		"format": {}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Codec != "h264" {
		t.Errorf("Codec = %q, want h264 (audio stream must be skipped)", r.Codec)
	}
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — requirement 7: malformed JSON
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_InvalidJSON(t *testing.T) {
	_, err := ParseFfprobeOutput(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
	// Must not be ErrNoVideoStream — that's a distinct, more specific error.
	if errors.Is(err, ErrNoVideoStream) {
		t.Error("malformed JSON must not produce ErrNoVideoStream")
	}
}

func TestParseFfprobeOutput_EmptyInput(t *testing.T) {
	_, err := ParseFfprobeOutput(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseFfprobeOutput_NullStreams(t *testing.T) {
	// Truncated / partially valid JSON — must not panic.
	_, err := ParseFfprobeOutput(strings.NewReader(`{"streams": null, "format": {}}`))
	// null streams → no video stream found
	if !errors.Is(err, ErrNoVideoStream) {
		t.Errorf("null streams should produce ErrNoVideoStream, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// sanitizeProbeError — requirement 8
// ---------------------------------------------------------------------------

func TestSanitizeProbeError_Generic(t *testing.T) {
	rawURI := "rtsp://admin:secret@192.168.1.100:554/stream1"
	msg := "Connection refused: rtsp://admin:secret@192.168.1.100:554/stream1"
	got := sanitizeProbeError(msg, rawURI)
	if strings.Contains(got, "secret") {
		t.Errorf("sanitizeProbeError left credentials in output: %q", got)
	}
	if !strings.Contains(got, "<uri>") {
		t.Errorf("sanitizeProbeError did not insert <uri> placeholder: %q", got)
	}
}

func TestSanitizeProbeError_SpecificURI(t *testing.T) {
	// Exact URI from the requirement specification.
	rawURI := "rtsp://user:secret@192.0.2.1/live"
	msg := "Connection to rtsp://user:secret@192.0.2.1/live failed: no route to host"
	got := sanitizeProbeError(msg, rawURI)
	if strings.Contains(got, "secret") {
		t.Errorf("password 'secret' must not appear in sanitized error: %q", got)
	}
	if strings.Contains(got, "user:secret@") {
		t.Errorf("user:pass@ pattern must not appear in sanitized error: %q", got)
	}
	if !strings.Contains(got, "<uri>") {
		t.Errorf("sanitized error must contain <uri> placeholder: %q", got)
	}
}

func TestSanitizeProbeError_EmptyURI(t *testing.T) {
	msg := "some error"
	got := sanitizeProbeError(msg, "")
	if got != msg {
		t.Errorf("sanitizeProbeError with empty URI should return msg unchanged, got %q", got)
	}
}

func TestSanitizeProbeError_URIAppearsMultipleTimes(t *testing.T) {
	rawURI := "rtsp://admin:pw@host/s"
	msg := rawURI + " retry " + rawURI
	got := sanitizeProbeError(msg, rawURI)
	if strings.Contains(got, "pw") {
		t.Errorf("all occurrences of URI must be redacted: %q", got)
	}
}

// ---------------------------------------------------------------------------
// buildFfprobeArgs
// ---------------------------------------------------------------------------

func TestBuildFfprobeArgs_TCPTransport(t *testing.T) {
	args := buildFfprobeArgs("rtsp://host/stream", "tcp")
	for i, a := range args {
		if a == "-rtsp_transport" && i+1 < len(args) && args[i+1] == "tcp" {
			return
		}
	}
	t.Errorf("expected -rtsp_transport tcp in args: %v", args)
}

func TestBuildFfprobeArgs_HTTPTransport(t *testing.T) {
	// http maps to -rtsp_transport tcp
	args := buildFfprobeArgs("rtsp://host/stream", "http")
	for i, a := range args {
		if a == "-rtsp_transport" && i+1 < len(args) && args[i+1] == "tcp" {
			return
		}
	}
	t.Errorf("expected -rtsp_transport tcp for http transport, args: %v", args)
}

func TestBuildFfprobeArgs_UDPTransport(t *testing.T) {
	args := buildFfprobeArgs("rtsp://host/stream", "udp")
	for i, a := range args {
		if a == "-rtsp_transport" && i+1 < len(args) && args[i+1] == "udp" {
			return
		}
	}
	t.Errorf("expected -rtsp_transport udp in args: %v", args)
}

func TestBuildFfprobeArgs_RTSPTransport(t *testing.T) {
	// "rtsp" transport must not inject -rtsp_transport flag
	args := buildFfprobeArgs("rtsp://host/stream", "rtsp")
	for _, a := range args {
		if a == "-rtsp_transport" {
			t.Errorf("rtsp transport must not add -rtsp_transport flag, got args: %v", args)
		}
	}
}

func TestBuildFfprobeArgs_URIIsLast(t *testing.T) {
	uri := "rtsp://host/stream"
	for _, transport := range []string{"rtsp", "tcp", "udp", "http"} {
		args := buildFfprobeArgs(uri, transport)
		if args[len(args)-1] != uri {
			t.Errorf("transport=%s: URI must be last arg, got %q", transport, args[len(args)-1])
		}
	}
}

func TestBuildFfprobeArgs_PrintFormatJSON(t *testing.T) {
	args := buildFfprobeArgs("rtsp://host/stream", "tcp")
	for i, a := range args {
		if a == "-print_format" && i+1 < len(args) && args[i+1] == "json" {
			return
		}
	}
	t.Errorf("expected -print_format json in args: %v", args)
}

// ---------------------------------------------------------------------------
// ParseFfprobeOutput — null/missing bit_rate in format
// ---------------------------------------------------------------------------

func TestParseFfprobeOutput_NullBitrate(t *testing.T) {
	// bit_rate field is JSON null — must still return WORKS=yes (no error) with BitrateBps=0.
	input := `{
		"streams": [
			{
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"avg_frame_rate": "25/1"
			}
		],
		"format": {
			"bit_rate": null
		}
	}`
	r, err := ParseFfprobeOutput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", r.Codec)
	}
	if r.BitrateBps != 0 {
		t.Errorf("BitrateBps = %d, want 0 when bit_rate is null", r.BitrateBps)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty — null bit_rate must not be treated as failure", r.Error)
	}
}

// ---------------------------------------------------------------------------
// InjectRTSPCredentials
// ---------------------------------------------------------------------------

func TestInjectRTSPCredentials_Basic(t *testing.T) {
	got, err := InjectRTSPCredentials("rtsp://192.0.2.1:554/1/1", "ha", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rtsp://ha:secret@192.0.2.1:554/1/1" {
		t.Errorf("got %q, want rtsp://ha:secret@192.0.2.1:554/1/1", got)
	}
}

func TestInjectRTSPCredentials_PreservesPathAndQuery(t *testing.T) {
	raw := "rtsp://192.168.0.1:554/live/stream?channel=1&subtype=0"
	got, err := InjectRTSPCredentials(raw, "admin", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "/live/stream") {
		t.Errorf("path not preserved: %q", got)
	}
	if !strings.Contains(got, "channel=1&subtype=0") {
		t.Errorf("query string not preserved: %q", got)
	}
}

func TestInjectRTSPCredentials_SpecialCharPassword(t *testing.T) {
	// Password contains @ and ! — must be URL-encoded so the URI remains valid.
	got, err := InjectRTSPCredentials("rtsp://host/stream", "user", "p@ss!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The literal '@' must not appear in the userinfo segment unencoded.
	if strings.Contains(got, "user:p@ss!@host") {
		t.Errorf("@ in password was not encoded: %q", got)
	}
	// Should contain the percent-encoded form.
	if !strings.Contains(got, "p%40ss") {
		t.Errorf("expected %%40 encoding of @ in password: %q", got)
	}
}

func TestInjectRTSPCredentials_ReplacesExistingUserinfo(t *testing.T) {
	// URI already has stale credentials — they must be replaced, not appended.
	raw := "rtsp://olduser:oldpass@host/stream"
	got, err := InjectRTSPCredentials(raw, "newuser", "newpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "olduser") || strings.Contains(got, "oldpass") {
		t.Errorf("old credentials must be replaced: %q", got)
	}
	if !strings.Contains(got, "newuser:newpass@") {
		t.Errorf("new credentials not found: %q", got)
	}
}

func TestInjectRTSPCredentials_SanitizationNeverLeaksPassword(t *testing.T) {
	// Simulate ffprobe writing the credential URI to stderr.
	// sanitizeProbeError must remove the credential URI.
	probeURI := "rtsp://ha:testpass@192.0.2.1:554/1/1"
	stderr := "rtsp://ha:testpass@192.0.2.1:554/1/1: Connection refused"
	sanitized := sanitizeProbeError(stderr, probeURI)
	if strings.Contains(sanitized, "testpass") {
		t.Errorf("sanitizeProbeError leaked password: %q", sanitized)
	}
	if strings.Contains(sanitized, "ha:") {
		t.Errorf("sanitizeProbeError leaked user:pass pattern: %q", sanitized)
	}
	if !strings.Contains(sanitized, "<uri>") {
		t.Errorf("sanitizeProbeError must replace URI with <uri>: %q", sanitized)
	}
}
