package platform

import "runtime"

var (
	IsMac   = runtime.GOOS == "darwin"
	IsLinux = runtime.GOOS == "linux"
)

func OS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}
