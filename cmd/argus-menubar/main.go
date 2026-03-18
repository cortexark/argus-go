// argus-menubar — macOS menu bar app for Argus AI Privacy Monitor.
// Shows a persistent icon in the menu bar with start/stop, reports,
// login toggle, and uninstall options.
package main

import (
	"github.com/cortexark/argus/internal/config"
	"github.com/cortexark/argus/internal/menubar"
)

func main() {
	cfg := config.Load()
	menubar.Run(cfg)
}
