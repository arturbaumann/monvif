package cmd

import "testing"

func TestValidateTransport(t *testing.T) {
	for _, valid := range []string{"rtsp", "tcp", "http", "udp"} {
		if err := validateTransport(valid); err != nil {
			t.Errorf("validateTransport(%q) unexpected error: %v", valid, err)
		}
	}
	for _, bad := range []string{"", "RTSP", "TCP", "rtp", "ftp", "auto"} {
		if err := validateTransport(bad); err == nil {
			t.Errorf("validateTransport(%q) expected error, got nil", bad)
		}
	}
}

func TestRedactURICredentials(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"rtsp://192.168.1.1/stream1", "rtsp://192.168.1.1/stream1"},
		{"rtsp://admin:secret@192.168.1.1/stream1", "rtsp://***@192.168.1.1/stream1"},
		{"rtsp://admin@192.168.1.1/stream1", "rtsp://***@192.168.1.1/stream1"},
		{"%%invalid%%", "%%invalid%%"},
	}
	for _, c := range cases {
		got := redactURICredentials(c.in)
		if got != c.want {
			t.Errorf("redactURICredentials(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
