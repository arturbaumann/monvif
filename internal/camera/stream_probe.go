package camera

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
)

// ErrNoVideoStream is returned by ParseFfprobeOutput when the ffprobe output
// contains no video stream.
var ErrNoVideoStream = errors.New("no video stream found")

// StreamProbeResult holds the result of probing a single stream via ffprobe.
// Error is non-empty if the probe failed; all other fields are zero values in that case.
type StreamProbeResult struct {
	Codec      string  `json:"codec,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	FPS        float64 `json:"fps,omitempty"`
	BitrateBps int64   `json:"bitrate_bps,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// ProbeStream runs ffprobe against rawURI and returns decoded stream metadata.
// rawURI must never appear in the returned Error string; credentials are sanitized.
func ProbeStream(ctx context.Context, rawURI string, transport string) StreamProbeResult {
	args := buildFfprobeArgs(rawURI, transport)
	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		return StreamProbeResult{Error: classifyProbeError(ctx, err, stderr.String(), rawURI)}
	}

	result, parseErr := ParseFfprobeOutput(bytes.NewReader(stdout))
	if parseErr != nil {
		if errors.Is(parseErr, ErrNoVideoStream) {
			return StreamProbeResult{Error: ErrNoVideoStream.Error()}
		}
		return StreamProbeResult{Error: "ffprobe: could not parse output"}
	}
	return result
}

// classifyProbeError maps a raw ffprobe execution failure to a human-readable
// message. It sanitizes rawURI out of any stderr content before returning.
func classifyProbeError(ctx context.Context, err error, stderr, rawURI string) string {
	// Killed by context deadline = probe timeout.
	if ctx.Err() != nil {
		return "ffprobe timed out"
	}

	// Binary not found on PATH.
	if errors.Is(err, exec.ErrNotFound) {
		return "ffprobe not found in $PATH: install ffprobe (https://ffmpeg.org/download)"
	}

	// Inspect sanitized stderr for common RTSP error patterns.
	sanitized := strings.TrimSpace(sanitizeProbeError(stderr, rawURI))
	lower := strings.ToLower(sanitized)
	switch {
	case strings.Contains(lower, "401") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "authentication failed"):
		return "RTSP auth failed"
	case strings.Contains(lower, "connection timed out") ||
		strings.Contains(lower, "operation timed out"):
		return "RTSP connection timed out"
	case strings.Contains(lower, "connection refused"):
		return "RTSP connection refused"
	case strings.Contains(lower, "no route to host"):
		return "RTSP: no route to host"
	case sanitized != "":
		return sanitized
	}

	return err.Error()
}

func buildFfprobeArgs(uri, transport string) []string {
	args := []string{"-v", "error", "-print_format", "json", "-show_streams", "-show_format"}
	switch strings.ToLower(transport) {
	case "tcp", "http":
		args = append(args, "-rtsp_transport", "tcp")
	case "udp":
		args = append(args, "-rtsp_transport", "udp")
	}
	return append(args, uri)
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	RFrameRate   string `json:"r_frame_rate"`
}

type ffprobeFormat struct {
	BitRate string `json:"bit_rate"`
}

// ParseFfprobeOutput decodes ffprobe JSON output into a StreamProbeResult.
// Exported so tests can call it directly without spawning a process.
func ParseFfprobeOutput(r io.Reader) (StreamProbeResult, error) {
	var out ffprobeOutput
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return StreamProbeResult{}, fmt.Errorf("json decode: %w", err)
	}

	var result StreamProbeResult
	for _, s := range out.Streams {
		if s.CodecType != "video" {
			continue
		}
		result.Codec = s.CodecName
		result.Width = s.Width
		result.Height = s.Height
		fps := ParseFraction(s.AvgFrameRate)
		if fps == 0 {
			fps = ParseFraction(s.RFrameRate)
		}
		result.FPS = fps
		break
	}

	if result.Codec == "" {
		return StreamProbeResult{}, ErrNoVideoStream
	}

	if out.Format.BitRate != "" {
		if bps, err := strconv.ParseInt(strings.TrimSpace(out.Format.BitRate), 10, 64); err == nil {
			result.BitrateBps = bps
		}
	}
	return result, nil
}

// ParseFraction parses a fraction string like "30/1" or "30000/1001".
// Exported for testing.
func ParseFraction(s string) float64 {
	if s == "" || s == "0/0" {
		return 0
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 1 {
		v, _ := strconv.ParseFloat(parts[0], 64)
		return v
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

// InjectRTSPCredentials returns a copy of rawURI with user and password set in
// the userinfo component. Any existing userinfo is replaced. The password is
// URL-encoded by url.UserPassword so special characters (e.g. @, !) are safe.
// Exported for testing.
func InjectRTSPCredentials(rawURI, user, password string) (string, error) {
	u, err := url.Parse(rawURI)
	if err != nil {
		return rawURI, fmt.Errorf("parse RTSP URI: %w", err)
	}
	if user != "" {
		u.User = url.UserPassword(user, password)
	}
	return u.String(), nil
}

// sanitizeProbeError replaces rawURI in an ffprobe error message with <uri>
// so credentials embedded in RTSP URIs are not exposed.
func sanitizeProbeError(msg, rawURI string) string {
	if rawURI != "" {
		msg = strings.ReplaceAll(msg, rawURI, "<uri>")
	}
	return msg
}
