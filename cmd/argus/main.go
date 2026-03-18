// Argus — AI Privacy Monitor
// Single binary, zero customer dependencies.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/cortexark/argus/internal/config"
	"github.com/cortexark/argus/internal/daemon"
	"github.com/cortexark/argus/internal/db"
	"github.com/cortexark/argus/internal/ipc"
	"github.com/cortexark/argus/internal/menubar"
	"github.com/cortexark/argus/internal/monitor"
	"github.com/cortexark/argus/internal/report"
	"github.com/cortexark/argus/internal/web"
)

var cfg *config.Config

func main() {
	cfg = config.Load()

	root := &cobra.Command{
		Use:   "argus",
		Short: "AI Privacy Monitor — watch what your AI tools access",
		Long:  "Argus monitors file access, network connections, and activity of AI applications on your machine.",
	}

	root.AddCommand(
		cmdStart(),
		cmdStop(),
		cmdStatus(),
		cmdMenubar(),
		cmdReport(),
		cmdLogs(),
		cmdDaemon(),
		cmdScan(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// cmdStart starts the Argus daemon via launchd/systemd.
func cmdStart() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Argus daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := daemon.New(cfg)
			return mgr.Start()
		},
	}
}

// cmdStop stops the daemon.
func cmdStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the Argus daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := daemon.New(cfg)
			return mgr.Stop()
		},
	}
}

// cmdStatus shows daemon status.
func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := ipc.NewClient(cfg.IPCSocketPath)
			if client.IsRunning() {
				color.Green("Argus daemon is running")
				resp, err := client.Send("status", nil)
				if err == nil {
					fmt.Println(string(resp.Data))
				}
			} else {
				color.Yellow("Argus daemon is not running")
				fmt.Println("Run: argus start")
			}
			return nil
		},
	}
}

// cmdReport generates a daily report.
func cmdReport() *cobra.Command {
	var dateStr string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate activity report",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer store.Close()

			date := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid date (use YYYY-MM-DD): %w", err)
				}
				date = parsed
			}

			gen := report.New(store)
			gen.PrintDailySummary(date)
			return nil
		},
	}
	cmd.Flags().StringVarP(&dateStr, "date", "d", "", "Date for report (YYYY-MM-DD, default: today)")
	return cmd
}

// cmdLogs shows recent file access alerts.
func cmdLogs() *cobra.Command {
	var limit int
	var sinceStr string
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show recent activity logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer store.Close()

			var since time.Time
			if sinceStr != "" {
				// Try multiple formats
				formats := []string{
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02",
				}
				for _, f := range formats {
					t, err := time.ParseInLocation(f, sinceStr, time.Local)
					if err == nil {
						since = t
						break
					}
				}
				if since.IsZero() {
					return fmt.Errorf("invalid --since format (use YYYY-MM-DDTHH:MM:SS)")
				}
			}

			gen := report.New(store)
			gen.PrintLiveFeed(since, limit)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Number of events to show")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Show events since this time (YYYY-MM-DDTHH:MM:SS)")
	return cmd
}

// cmdDaemon contains internal daemon subcommands.
func cmdDaemon() *cobra.Command {
	daemonCmd := &cobra.Command{
		Use:    "daemon",
		Short:  "Daemon management (internal)",
		Hidden: true,
	}
	daemonCmd.AddCommand(cmdDaemonRun())
	return daemonCmd
}

// cmdDaemonRun is the long-running daemon process (called by launchd/systemd).
func cmdDaemonRun() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the daemon process (called by init system)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}
}

// cmdMenubar launches the macOS menu bar app.
func cmdMenubar() *cobra.Command {
	return &cobra.Command{
		Use:   "menubar",
		Short: "Launch macOS menu bar app",
		RunE: func(cmd *cobra.Command, args []string) error {
			menubar.Run(cfg)
			return nil
		},
	}
}

// cmdScan runs a one-shot scan and prints results.
func cmdScan() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Run a one-shot scan and print results",
		RunE: func(cmd *cobra.Command, args []string) error {
			procs, err := monitor.ScanProcesses()
			if err != nil {
				return err
			}

			bold := color.New(color.Bold)
			bold.Println("\n=== ARGUS ONE-SHOT SCAN ===\n")

			found := 0
			for _, p := range procs {
				if p.Name == "" {
					continue
				}
				// Use classifier to filter AI processes
				// (simplified: just show all for scan output)
				_ = p
				found++
			}

			color.Cyan("Processes scanned: %d\n", len(procs))

			conns, err := monitor.ScanNetwork()
			if err != nil {
				color.Yellow("Network scan warning: %v\n", err)
			} else {
				color.Cyan("Network connections: %d\n", len(conns))
			}

			fmt.Println("\nRun `argus report` for a full summary of stored activity.")
			return nil
		},
	}
}

// runDaemon is the long-running daemon loop.
func runDaemon() error {
	if err := os.MkdirAll(cfg.LogDir, 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer store.Close()

	// Start monitors
	procScanner := monitor.NewProcessScanner(store, cfg.ScanInterval)
	netMonitor := monitor.NewNetworkMonitor(store, cfg.ScanInterval)
	fileMonitor := monitor.NewFileMonitor(store, cfg.ScanInterval)
	injDetector := monitor.NewInjectionDetector(store, 60*time.Second)

	procScanner.Start()
	netMonitor.Start()
	fileMonitor.Start()
	injDetector.Start()

	// Start web dashboard
	webServer := web.New(store, cfg.WebPort)
	if err := webServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Web server warning: %v\n", err)
	}

	// Start IPC server
	ipcServer := ipc.NewServer(cfg.IPCSocketPath)
	ipcServer.Register("status", func(msg ipc.Message) ipc.Message {
		data, _ := json.Marshal(map[string]any{
			"running": true,
			"dbPath":  cfg.DBPath,
			"webPort": cfg.WebPort,
		})
		return ipc.Message{Command: "status", Data: data}
	})
	ipcServer.Register("ping", func(msg ipc.Message) ipc.Message {
		return ipc.Message{Command: "pong"}
	})

	if err := ipcServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "IPC server warning: %v\n", err)
	}

	fmt.Printf("Argus daemon running (db: %s, web: http://127.0.0.1:%d)\n",
		cfg.DBPath, cfg.WebPort)

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	fmt.Println("Shutting down...")
	procScanner.Stop()
	netMonitor.Stop()
	fileMonitor.Stop()
	injDetector.Stop()
	webServer.Stop()
	ipcServer.Stop()

	return nil
}
