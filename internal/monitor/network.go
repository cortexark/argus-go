package monitor

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cortexark/argus/internal/aiapps"
	"github.com/cortexark/argus/internal/db"
	"github.com/cortexark/argus/pkg/platform"
)

// NetworkConn represents a single network connection.
type NetworkConn struct {
	Protocol    string
	LocalAddr   string
	LocalPort   int
	RemoteAddr  string
	RemotePort  int
	State       string
	PID         int
	ProcessName string
}

// ScanNetwork returns current TCP/UDP connections.
func ScanNetwork() ([]NetworkConn, error) {
	if platform.IsLinux {
		return scanSS()
	}
	return scanNetstat()
}

// scanNetstat uses `netstat -anv` on macOS.
func scanNetstat() ([]NetworkConn, error) {
	cmd := exec.Command("netstat", "-anv", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netstat failed: %w", err)
	}
	conns := parseNetstat(out)

	// Also scan lsof for PID->connection mapping
	lsofMap, _ := scanLsofPorts()
	for i := range conns {
		if pid, ok := lsofMap[conns[i].LocalPort]; ok {
			conns[i].PID = pid
		}
	}
	return conns, nil
}

// parseNetstat parses macOS `netstat -anv` output.
func parseNetstat(data []byte) []NetworkConn {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var conns []NetworkConn
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// tcp4  0  0  127.0.0.1.8080  *.*  LISTEN
		if len(fields) < 6 {
			continue
		}
		proto := fields[0]
		if !strings.HasPrefix(proto, "tcp") && !strings.HasPrefix(proto, "udp") {
			continue
		}
		local := fields[3]
		remote := fields[4]
		state := ""
		if len(fields) > 5 {
			state = fields[5]
		}

		lAddr, lPort := splitAddrPort(local)
		rAddr, rPort := splitAddrPort(remote)

		conns = append(conns, NetworkConn{
			Protocol:   proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
			State:      state,
		})
	}
	return conns
}

// scanSS uses `ss -tunap` on Linux.
func scanSS() ([]NetworkConn, error) {
	cmd := exec.Command("ss", "-tunap")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ss failed: %w", err)
	}
	return parseSS(out), nil
}

// parseSS parses `ss -tunap` output.
func parseSS(data []byte) []NetworkConn {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var conns []NetworkConn
	// Skip header
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		// Netid  State  Recv-Q  Send-Q  Local  Peer  Process
		if len(fields) < 5 {
			continue
		}
		proto := fields[0]
		state := fields[1]
		local := fields[4]
		remote := fields[5]

		lAddr, lPort := splitAddrPort(local)
		rAddr, rPort := splitAddrPort(remote)

		pid := 0
		if len(fields) > 6 {
			pid = extractPID(fields[6])
		}

		conns = append(conns, NetworkConn{
			Protocol:   proto,
			LocalAddr:  lAddr,
			LocalPort:  lPort,
			RemoteAddr: rAddr,
			RemotePort: rPort,
			State:      state,
			PID:        pid,
		})
	}
	return conns
}

// splitAddrPort splits "127.0.0.1:8080" or "127.0.0.1.8080" (macOS dot notation).
func splitAddrPort(s string) (string, int) {
	// IPv6 [::1]:8080
	if strings.HasPrefix(s, "[") {
		if idx := strings.LastIndex(s, "]:"); idx >= 0 {
			port, _ := strconv.Atoi(s[idx+2:])
			return s[1:idx], port
		}
		return s, 0
	}
	// macOS dot notation: 127.0.0.1.8080
	// Count dots: if 4 dots, last segment is port
	parts := strings.Split(s, ".")
	if len(parts) == 5 {
		port, err := strconv.Atoi(parts[4])
		if err == nil {
			return strings.Join(parts[:4], "."), port
		}
	}
	// Standard colon notation
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		port, _ := strconv.Atoi(s[idx+1:])
		return s[:idx], port
	}
	return s, 0
}

// extractPID extracts PID from ss process column like `users:(("node",pid=1234,fd=5))`
func extractPID(s string) int {
	if idx := strings.Index(s, "pid="); idx >= 0 {
		rest := s[idx+4:]
		end := strings.IndexAny(rest, ",)")
		if end > 0 {
			pid, _ := strconv.Atoi(rest[:end])
			return pid
		}
	}
	return 0
}

// scanLsofPorts returns a map of localPort -> PID using lsof (macOS fallback).
func scanLsofPorts() (map[int]int, error) {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-n", "-P")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	result := map[int]int{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Scan() // header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 9 {
			continue
		}
		pid, _ := strconv.Atoi(fields[1])
		_, port := splitAddrPort(fields[8])
		if port > 0 {
			result[port] = pid
		}
	}
	return result, nil
}

// NetworkMonitor continuously scans network connections.
type NetworkMonitor struct {
	store    *db.Store
	interval time.Duration
	done     chan struct{}
}

// NewNetworkMonitor creates a network monitor.
func NewNetworkMonitor(store *db.Store, interval time.Duration) *NetworkMonitor {
	return &NetworkMonitor{
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins monitoring in a goroutine.
func (nm *NetworkMonitor) Start() {
	go nm.loop()
}

// Stop signals the monitor to stop.
func (nm *NetworkMonitor) Stop() {
	close(nm.done)
}

func (nm *NetworkMonitor) loop() {
	nm.scan()
	ticker := time.NewTicker(nm.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			nm.scan()
		case <-nm.done:
			return
		}
	}
}

func (nm *NetworkMonitor) scan() {
	conns, err := ScanNetwork()
	if err != nil {
		return
	}

	pidNames := getPIDNames()

	for _, c := range conns {
		// Only log connections to AI endpoints or from known AI pids
		endpoint := fmt.Sprintf("%s:%d", c.RemoteAddr, c.RemotePort)
		label := aiapps.MatchEndpoint(c.RemoteAddr)
		if label == "" && c.PID == 0 {
			continue
		}

		procName := c.ProcessName
		if procName == "" {
			procName = pidNames[c.PID]
		}

		event := db.NetworkEvent{
			PID:         c.PID,
			ProcessName: procName,
			Protocol:    c.Protocol,
			LocalPort:   c.LocalPort,
			RemoteAddr:  c.RemoteAddr,
			RemotePort:  c.RemotePort,
			Endpoint:    endpoint,
			Label:       label,
			State:       c.State,
		}
		_ = nm.store.InsertNetworkEvent(event)
	}
}

// getPIDNames returns a snapshot map of PID -> process name from `ps`.
func getPIDNames() map[int]string {
	cmd := exec.Command("ps", "-eo", "pid,comm")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	result := map[int]string{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Scan() // header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			pid, _ := strconv.Atoi(fields[0])
			result[pid] = fields[1]
		}
	}
	return result
}
