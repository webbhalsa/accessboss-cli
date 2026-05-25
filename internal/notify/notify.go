package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

func Send(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		exec.Command("osascript", "-e", script).Run() //nolint:errcheck
	case "linux":
		if _, err := exec.LookPath("notify-send"); err == nil {
			exec.Command("notify-send", title, message).Run() //nolint:errcheck
		}
	}
}
