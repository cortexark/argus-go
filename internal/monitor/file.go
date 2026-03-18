package monitor

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cortexark/argus/internal/aiapps"
	"github.com/cortexark/argus/internal/db"
	"github.com/cortexark/argus/pkg/platform"
)

// FileMonitor watches sensitive paths using fsnotify or polling.
type FileMonitor struct {
	store    *db.Store
	interval time.Duration
	done     chan struct{}
}

// NewFileMonitor creates a file monitor.
func NewFileMonitor(store *db.Store, interval time.Duration) *FileMonitor {
	return &FileMonitor{
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins monitoring in a goroutine.
func (fm *FileMonitor) Start() {
	go fm.loop()
}

// Stop signals the monitor to stop.
func (fm *FileMonitor) Stop() {
	close(fm.done)
}

func (fm *FileMonitor) loop() {
	fm.scan()
	ticker := time.NewTicker(fm.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fm.scan()
		case <-fm.done:
			return
		}
	}
}

// scan uses lsof (macOS) or /proc/PID/fd (Linux) to find open sensitive files.
func (fm *FileMonitor) scan() {
	var events []db.FileAccessEvent
	if platform.IsMac {
		events = fm.scanLsof()
	} else {
		events = fm.scanProc()
	}
	for _, e := range events {
		_ = fm.store.InsertFileAccessEvent(e)
	}
}

// scanLsof uses `lsof -F pcn` for fast parseable output.
func (fm *FileMonitor) scanLsof() []db.FileAccessEvent {
	// Build lsof command targeting sensitive paths
	args := []string{"-F", "pcn", "-n"}
	for _, paths := range aiapps.SensitivePaths {
		for _, p := range paths {
			args = append(args, "+D", p)
		}
	}

	cmd := exec.Command("lsof", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseLsofOutput(out)
}

// parseLsofOutput parses lsof -F pcn output.
// Format: p<pid>\nc<cmd>\nn<file>  (repeating groups)
func parseLsofOutput(data []byte) []db.FileAccessEvent {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var events []db.FileAccessEvent

	var pid, cmd string
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case 'p':
			pid = line[1:]
		case 'c':
			cmd = line[1:]
		case 'n':
			path := line[1:]
			if path == "" || path[0] == '/' {
				cat, _ := aiapps.ClassifyPath(path)
				if cat != "" {
					events = append(events, db.FileAccessEvent{
						ProcessName: cmd,
						FilePath:    path,
						Sensitivity: cat,
					})
					_ = pid // used indirectly via cmd
				}
			}
		}
	}
	return events
}

// scanProc reads /proc/PID/fd on Linux to find open sensitive files.
func (fm *FileMonitor) scanProc() []db.FileAccessEvent {
	cmd := exec.Command("find", "/proc", "-maxdepth", "3", "-path", "*/fd/*", "-follow", "-type", "f")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var events []db.FileAccessEvent
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		cat, _ := aiapps.ClassifyPath(path)
		if cat != "" {
			procName := extractProcName(path)
			events = append(events, db.FileAccessEvent{
				ProcessName: procName,
				FilePath:    path,
				Sensitivity: cat,
			})
		}
	}
	return events
}

// extractProcName reads /proc/PID/comm from a /proc/PID/fd/N path.
func extractProcName(fdPath string) string {
	// /proc/1234/fd/5 -> /proc/1234/comm
	parts := strings.Split(fdPath, "/")
	if len(parts) >= 4 {
		commPath := fmt.Sprintf("/proc/%s/comm", parts[2])
		out, err := exec.Command("cat", commPath).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "unknown"
}

// ScanOpenFiles returns sensitive files open by a specific PID.
func ScanOpenFiles(pid int) ([]string, error) {
	var cmd *exec.Cmd
	if platform.IsMac {
		cmd = exec.Command("lsof", "-p", fmt.Sprintf("%d", pid), "-F", "n", "-n")
	} else {
		cmd = exec.Command("ls", "-la", fmt.Sprintf("/proc/%d/fd", pid))
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("open files scan failed: %w", err)
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "n/") {
			path := line[1:]
			if cat, _ := aiapps.ClassifyPath(path); cat != "" {
				files = append(files, path)
			}
		}
	}
	return files, nil
}
