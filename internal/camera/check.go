package camera

import (
	"context"
	"fmt"
	"net"
	"time"
)

// CheckOptions controls which optional queries Check performs.
type CheckOptions struct {
	WithCapabilities bool
}

// CheckResult is the outcome of probing one camera.
type CheckResult struct {
	Name          string
	IP            string
	Port          int
	Reachable     bool
	Authenticated bool
	Info          DeviceInfo
	Caps          *Capabilities // nil when not requested or when the query failed
	Err           string
}

// Check dials host:port, queries device info, and optionally queries capabilities.
// It always returns a filled CheckResult; errors are recorded in Err.
func Check(ctx context.Context, name, host string, port int, username, password string, opts CheckOptions) CheckResult {
	r := CheckResult{Name: name, IP: host, Port: port}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		r.Err = fmt.Sprintf("unreachable: %v", err)
		return r
	}
	conn.Close()
	r.Reachable = true

	client, err := New(host, port, username, password)
	if err != nil {
		r.Err = fmt.Sprintf("connect: %v", err)
		return r
	}

	info, err := client.GetDeviceInfo(ctx)
	if err != nil {
		r.Err = fmt.Sprintf("query: %v", err)
		return r
	}

	r.Authenticated = true
	r.Info = info

	if opts.WithCapabilities {
		caps, err := client.GetCapabilities(ctx)
		if err != nil {
			r.Err = fmt.Sprintf("capabilities: %v", err)
		} else {
			r.Caps = &caps
		}
	}

	return r
}
