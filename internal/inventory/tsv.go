package inventory

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Camera is one row from the inventory file.
type Camera struct {
	Name string
	IP   string
	Port int
}

// ParseTSV reads name<TAB>ip<TAB>port rows, skipping blank lines and # comments.
func ParseTSV(r io.Reader) ([]Camera, error) {
	var cameras []Camera
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 tab-separated fields, got %d", lineNum, len(fields))
		}
		name := strings.TrimSpace(fields[0])
		ip := strings.TrimSpace(fields[1])
		portStr := strings.TrimSpace(fields[2])
		if name == "" || ip == "" {
			return nil, fmt.Errorf("line %d: name and ip must not be empty", lineNum)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("line %d: invalid port %q", lineNum, portStr)
		}
		cameras = append(cameras, Camera{Name: name, IP: ip, Port: port})
	}
	return cameras, scanner.Err()
}
