package menubar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"

	"github.com/cortexark/argus/internal/config"
	"github.com/cortexark/argus/internal/daemon"
	"github.com/cortexark/argus/internal/ipc"
)

// App holds the menu bar application state.
type App struct {
	cfg    *config.Config
	client *ipc.Client
	mgr    *daemon.Manager
}

// Run starts the menu bar event loop (blocks until quit).
func Run(cfg *config.Config) {
	app := &App{
		cfg:    cfg,
		client: ipc.NewClient(cfg.IPCSocketPath),
		mgr:    daemon.New(cfg),
	}
	systray.Run(app.onReady, app.onQuit)
}

func (a *App) onQuit() {}

func (a *App) onReady() {
	systray.SetIcon(IconActive())
	systray.SetTitle("")
	systray.SetTooltip("Argus — AI Privacy Monitor")

	// ── Status ────────────────────────────────────────────────────────
	mStatus := systray.AddMenuItem("● Checking...", "Daemon status")
	mStatus.Disable()

	systray.AddSeparator()

	// ── Start / Stop ──────────────────────────────────────────────────
	mToggle := systray.AddMenuItem("Start Monitoring", "Start or stop the Argus daemon")

	systray.AddSeparator()

	// ── Reports ───────────────────────────────────────────────────────
	mReport := systray.AddMenuItem("Show Report", "")
	mReport.Disable()

	m5min := systray.AddMenuItem("    Last 5 minutes", "Activity in the last 5 minutes")
	m10min := systray.AddMenuItem("    Last 10 minutes", "Activity in the last 10 minutes")
	m30min := systray.AddMenuItem("    Last 30 minutes", "Activity in the last 30 minutes")
	m1hr := systray.AddMenuItem("    Last 1 hour", "Activity in the last hour")
	m24hr := systray.AddMenuItem("    Last 24 hours", "Today's full activity")

	systray.AddSeparator()

	// ── Dashboard ─────────────────────────────────────────────────────
	mDashboard := systray.AddMenuItem("Open Dashboard", fmt.Sprintf("http://127.0.0.1:%d", a.cfg.WebPort))

	systray.AddSeparator()

	// ── Login / install ───────────────────────────────────────────────
	mLogin := systray.AddMenuItem("Start at Login: OFF", "Toggle run-at-login via launchd")

	systray.AddSeparator()

	// ── Uninstall ─────────────────────────────────────────────────────
	mUninstall := systray.AddMenuItem("Uninstall Argus", "Stop daemon and remove all Argus files")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Menu Bar App", "Close this menu bar icon (daemon keeps running)")

	// ── Periodic status refresh ───────────────────────────────────────
	isRunning := false
	loginEnabled := a.isLoginEnabled()
	a.refreshLogin(mLogin, loginEnabled)

	go func() {
		for {
			isRunning = a.client.IsRunning()
			a.refreshStatus(mStatus, mToggle, isRunning)
			time.Sleep(5 * time.Second)
		}
	}()

	// ── Event loop ────────────────────────────────────────────────────
	for {
		select {
		case <-mToggle.ClickedCh:
			if isRunning {
				_ = a.mgr.Stop()
				systray.SetIcon(IconPaused())
			} else {
				_ = a.mgr.Start()
				systray.SetIcon(IconActive())
			}

		case <-m5min.ClickedCh:
			a.openTerminalReport(5 * time.Minute)
		case <-m10min.ClickedCh:
			a.openTerminalReport(10 * time.Minute)
		case <-m30min.ClickedCh:
			a.openTerminalReport(30 * time.Minute)
		case <-m1hr.ClickedCh:
			a.openTerminalReport(1 * time.Hour)
		case <-m24hr.ClickedCh:
			a.openTerminalReport(24 * time.Hour)

		case <-mDashboard.ClickedCh:
			a.openBrowser(fmt.Sprintf("http://127.0.0.1:%d", a.cfg.WebPort))

		case <-mLogin.ClickedCh:
			loginEnabled = !loginEnabled
			a.setLoginEnabled(loginEnabled)
			a.refreshLogin(mLogin, loginEnabled)

		case <-mUninstall.ClickedCh:
			a.runUninstall()
			systray.Quit()
			return

		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (a *App) refreshStatus(mStatus, mToggle *systray.MenuItem, running bool) {
	if running {
		mStatus.SetTitle("● Argus is Running")
		mToggle.SetTitle("Stop Monitoring")
		systray.SetIcon(IconActive())
	} else {
		mStatus.SetTitle("○ Argus is Stopped")
		mToggle.SetTitle("Start Monitoring")
		systray.SetIcon(IconPaused())
	}
}

func (a *App) refreshLogin(mLogin *systray.MenuItem, enabled bool) {
	if enabled {
		mLogin.SetTitle("Start at Login: ON  ✓")
	} else {
		mLogin.SetTitle("Start at Login: OFF")
	}
}

// openTerminalReport opens a new Terminal window showing the report for `window` duration.
func (a *App) openTerminalReport(window time.Duration) {
	label := formatDuration(window)
	var script string

	if window >= 24*time.Hour {
		script = fmt.Sprintf(`tell application "Terminal"
    activate
    do script "argus report; echo ''; read -p 'Press Enter to close...';"
end tell`)
	} else {
		// Use argus logs --since flag
		sinceStr := time.Now().Add(-window).Format("2006-01-02T15:04:05")
		script = fmt.Sprintf(`tell application "Terminal"
    activate
    do script "argus logs --since %q; echo ''; echo '=== Last %s ==='; read -p 'Press Enter to close...';"
end tell`, sinceStr, label)
	}

	exec.Command("osascript", "-e", script).Start() //nolint:errcheck
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hr", int(d.Hours()))
}

// openBrowser opens a URL in the default browser.
func (a *App) openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start() //nolint:errcheck
	case "linux":
		exec.Command("xdg-open", url).Start() //nolint:errcheck
	}
}

// isLoginEnabled checks if the launchd plist is installed and loaded.
func (a *App) isLoginEnabled() bool {
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.argus.daemon.plist")
	_, err := os.Stat(plistPath)
	return err == nil
}

// setLoginEnabled installs or removes the launchd plist.
func (a *App) setLoginEnabled(enable bool) {
	if enable {
		_ = a.mgr.Start()
	} else {
		home, _ := os.UserHomeDir()
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.argus.daemon.plist")
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
	}
}

// runUninstall stops daemon and moves all Argus data to Trash.
func (a *App) runUninstall() {
	// Stop daemon
	_ = a.mgr.Stop()

	// Remove launchd plist
	home, _ := os.UserHomeDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.argus.daemon.plist")
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	// Move data to Trash (never rm -rf)
	argusDir := filepath.Join(home, ".argus")
	trash := filepath.Join(home, ".Trash", "argus-data-"+time.Now().Format("20060102-150405"))
	_ = os.Rename(argusDir, trash)

	a.notify("Argus Uninstalled",
		"Daemon stopped. Data moved to Trash. Delete the argus binary to finish.")
}

// notify sends a macOS notification.
func (a *App) notify(title, message string) {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	exec.Command("osascript", "-e", script).Start() //nolint:errcheck
}
