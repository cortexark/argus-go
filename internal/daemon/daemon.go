// Package daemon manages Argus as a persistent background service.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cortexark/argus/internal/config"
)

// Status returns current daemon status.
type Status struct {
	Running bool
	PID     int
	Uptime  time.Duration
}

// Manager handles start/stop/status for the daemon.
type Manager struct {
	cfg *config.Config
}

// New creates a daemon manager.
func New(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// Start launches the daemon process.
func (m *Manager) Start() error {
	if runtime.GOOS == "darwin" {
		return m.startLaunchd()
	}
	return m.startSystemd()
}

// Stop stops the daemon.
func (m *Manager) Stop() error {
	if runtime.GOOS == "darwin" {
		return m.stopLaunchd()
	}
	return m.stopSystemd()
}

// Status returns whether the daemon is running.
func (m *Manager) Status() (Status, error) {
	pid, err := m.readPID()
	if err != nil || pid == 0 {
		return Status{Running: false}, nil
	}

	// Check if process exists
	proc, err := os.FindProcess(pid)
	if err != nil {
		return Status{Running: false}, nil
	}
	if err := proc.Signal(os.Signal(nil)); err != nil {
		return Status{Running: false}, nil
	}

	return Status{Running: true, PID: pid}, nil
}

func (m *Manager) readPID() (int, error) {
	pidFile := filepath.Join(m.cfg.LogDir, "argus.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// startLaunchd installs and loads a macOS LaunchAgent.
func (m *Manager) startLaunchd() error {
	plistPath, err := m.writePlist()
	if err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// Unload first in case already registered
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	cmd := exec.Command("launchctl", "load", "-w", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", out, err)
	}
	fmt.Println("Argus daemon started (launchd)")
	return nil
}

func (m *Manager) stopLaunchd() error {
	plistPath := m.plistPath()
	cmd := exec.Command("launchctl", "unload", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload: %s: %w", out, err)
	}
	fmt.Println("Argus daemon stopped")
	return nil
}

func (m *Manager) plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.argus.daemon.plist")
}

func (m *Manager) writePlist() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Resolve symlinks so launchd gets the real path
	exe, _ = filepath.EvalSymlinks(exe)

	home, _ := os.UserHomeDir()
	logDir := m.cfg.LogDir
	plistPath := m.plistPath()

	laDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(laDir, 0755); err != nil {
		return "", err
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.argus.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>%s</string>
    </dict>
</dict>
</plist>`,
		exe,
		filepath.Join(logDir, "daemon.log"),
		filepath.Join(logDir, "daemon-error.log"),
		home,
	)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return "", err
	}
	return plistPath, nil
}

// startSystemd installs and starts a systemd user service on Linux.
func (m *Manager) startSystemd() error {
	unitPath, err := m.writeSystemdUnit()
	if err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}

	cmds := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "argus"},
		{"systemctl", "--user", "start", "argus"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s: %w", args, out, err)
		}
	}
	_ = unitPath
	fmt.Println("Argus daemon started (systemd)")
	return nil
}

func (m *Manager) stopSystemd() error {
	if out, err := exec.Command("systemctl", "--user", "stop", "argus").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop: %s: %w", out, err)
	}
	fmt.Println("Argus daemon stopped")
	return nil
}

func (m *Manager) writeSystemdUnit() (string, error) {
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)

	home, _ := os.UserHomeDir()
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return "", err
	}
	unitPath := filepath.Join(unitDir, "argus.service")

	unit := fmt.Sprintf(`[Unit]
Description=Argus AI Privacy Monitor
After=network.target

[Service]
ExecStart=%s daemon run
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
`, exe)

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return "", err
	}
	return unitPath, nil
}
