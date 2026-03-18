// Package report generates human-readable Argus activity reports.
package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"github.com/cortexark/argus/internal/db"
)

var (
	bold    = color.New(color.Bold)
	red     = color.New(color.FgRed, color.Bold)
	yellow  = color.New(color.FgYellow)
	green   = color.New(color.FgGreen)
	cyan    = color.New(color.FgCyan)
	magenta = color.New(color.FgMagenta)
)

// Generator creates reports from the store.
type Generator struct {
	store *db.Store
}

// New creates a report generator.
func New(store *db.Store) *Generator {
	return &Generator{store: store}
}

// PrintDailySummary prints a formatted daily summary to stdout.
func (g *Generator) PrintDailySummary(date time.Time) {
	summary, err := g.store.GetDailySummary(date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting summary: %v\n", err)
		return
	}

	bold.Printf("\n=== ARGUS DAILY REPORT — %s ===\n\n", date.Format("2006-01-02"))

	// Process activity
	cyan.Println("AI PROCESSES DETECTED")
	fmt.Println(strings.Repeat("─", 60))

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"App", "Confidence", "Connections", "File Accesses"})

	for _, row := range summary.Processes {
		conf := row.Confidence
		switch conf {
		case "CONFIRMED_AI":
			conf = "CONFIRMED"
		case "LIKELY_AI":
			conf = "LIKELY"
		}
		_ = table.Append([]string{
			row.AppName,
			conf,
			fmt.Sprintf("%d", row.NetworkCount),
			fmt.Sprintf("%d", row.FileCount),
		})
	}
	_ = table.Render()

	// File access alerts
	if len(summary.FileAlerts) > 0 {
		fmt.Println()
		red.Println("SENSITIVE FILE ACCESS")
		fmt.Println(strings.Repeat("─", 60))

		for _, f := range summary.FileAlerts {
			sensitivity := f.Sensitivity
			switch sensitivity {
			case "credentials":
				red.Printf("  [!] %s  →  %s  (%s)\n", f.ProcessName, f.FilePath, sensitivity)
			case "browserData":
				yellow.Printf("  [!] %s  →  %s  (%s)\n", f.ProcessName, f.FilePath, sensitivity)
			default:
				fmt.Printf("  [*] %s  →  %s  (%s)\n", f.ProcessName, f.FilePath, sensitivity)
			}
		}
	}

	// Network activity
	if len(summary.NetworkEvents) > 0 {
		fmt.Println()
		magenta.Println("NETWORK CONNECTIONS TO AI ENDPOINTS")
		fmt.Println(strings.Repeat("─", 60))

		for _, n := range summary.NetworkEvents {
			label := n.Label
			if label == "" {
				label = n.RemoteAddr
			}
			fmt.Printf("  %s  →  %s:%d  [%s]\n",
				n.ProcessName, label, n.RemotePort, n.Protocol)
		}
	}

	// Injection alerts
	if len(summary.InjectionAlerts) > 0 {
		fmt.Println()
		red.Println("⚠  PROMPT INJECTION ATTEMPTS DETECTED")
		fmt.Println(strings.Repeat("─", 60))

		for _, a := range summary.InjectionAlerts {
			red.Printf("  [%s] Source: %s\n", strings.ToUpper(a.Severity), a.Source)
			fmt.Printf("    Pattern: %s\n", a.Pattern)
			if a.Snippet != "" {
				fmt.Printf("    Context: %s\n", truncate(a.Snippet, 100))
			}
		}
	}

	// Footer
	fmt.Println()
	green.Printf("Report generated at %s\n\n", time.Now().Format("15:04:05"))
}

// PrintLiveFeed prints the most recent events in a live-feed style.
func (g *Generator) PrintLiveFeed(limit int) {
	alerts, err := g.store.GetRecentFileAlerts(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	bold.Println("\n=== ARGUS LIVE FEED ===\n")

	if len(alerts) == 0 {
		green.Println("  No alerts in recent activity.")
		return
	}

	for _, a := range alerts {
		ts := a.Timestamp.Format("15:04:05")
		switch a.Sensitivity {
		case "credentials":
			red.Printf("[%s] CRED  %s accessed %s\n", ts, a.ProcessName, a.FilePath)
		case "browserData":
			yellow.Printf("[%s] BROWSER  %s accessed %s\n", ts, a.ProcessName, a.FilePath)
		default:
			fmt.Printf("[%s] FILE  %s accessed %s (%s)\n", ts, a.ProcessName, a.FilePath, a.Sensitivity)
		}
	}
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
