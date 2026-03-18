// Package monitor contains all Argus monitoring subsystems.
package monitor

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cortexark/argus/internal/classifier"
	"github.com/cortexark/argus/internal/db"
	"github.com/cortexark/argus/pkg/platform"
)

// ProcessInfo holds a snapshot of one running process.
type ProcessInfo struct {
	PID        int
	PPID       int
	Name       string
	Cmdline    string
	PPIDName   string
	User       string
	CPUPercent float64
	MemMB      float64
}

// ScanProcesses runs ps and returns AI-relevant processes.
func ScanProcesses() ([]ProcessInfo, error) {
	var cmd *exec.Cmd
	if platform.IsMac {
		cmd = exec.Command("ps", "-eo", "pid,ppid,user,%cpu,rss,comm,command")
	} else {
		cmd = exec.Command("ps", "-eo", "pid,ppid,user,%cpu,rss,comm,args")
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	return parsePS(out), nil
}

func parsePS(data []byte) []ProcessInfo {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var procs []ProcessInfo
	pidMap := map[int]string{} // pid -> name for ancestry lookup

	// Skip header
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		user := fields[2]
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		rss, _ := strconv.ParseFloat(fields[4], 64)
		name := filepath(fields[5])
		cmdline := strings.Join(fields[6:], " ")

		pidMap[pid] = name

		info := ProcessInfo{
			PID:        pid,
			PPID:       ppid,
			Name:       name,
			Cmdline:    cmdline,
			User:       user,
			CPUPercent: cpu,
			MemMB:      rss / 1024,
		}
		procs = append(procs, info)
	}

	// Second pass: fill PPIDName
	for i := range procs {
		procs[i].PPIDName = pidMap[procs[i].PPID]
	}

	return procs
}

// filepath extracts the base name from a full path.
func filepath(s string) string {
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// ProcessScanner continuously scans processes and stores AI snapshots.
type ProcessScanner struct {
	store    *db.Store
	interval time.Duration
	done     chan struct{}
}

// NewProcessScanner creates a scanner with the given scan interval.
func NewProcessScanner(store *db.Store, interval time.Duration) *ProcessScanner {
	return &ProcessScanner{
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins scanning in a goroutine.
func (ps *ProcessScanner) Start() {
	go ps.loop()
}

// Stop signals the scanner to stop.
func (ps *ProcessScanner) Stop() {
	close(ps.done)
}

func (ps *ProcessScanner) loop() {
	ps.scan()
	ticker := time.NewTicker(ps.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ps.scan()
		case <-ps.done:
			return
		}
	}
}

func (ps *ProcessScanner) scan() {
	procs, err := ScanProcesses()
	if err != nil {
		return
	}

	for _, p := range procs {
		result := classifier.Classify(p.Name, p.Cmdline, p.PPIDName, nil, nil)
		if result.Confidence == classifier.NotAI {
			continue
		}

		appName := p.Name
		if result.App != nil {
			appName = result.App.Label
		}

		snap := db.ProcessSnapshot{
			PID:        p.PID,
			Name:       p.Name,
			AppName:    appName,
			Cmdline:    p.Cmdline,
			User:       p.User,
			CPUPercent: p.CPUPercent,
			MemMB:      p.MemMB,
			Confidence: result.Confidence,
			Score:      result.Score,
		}
		_ = ps.store.InsertProcessSnapshot(snap)
	}
}
