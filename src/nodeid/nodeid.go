package nodeid

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"runtime"
	"strings"
)

// GetNodeID generates a consistent unique node ID based on system-specific data.
func GetNodeID() string {
	var parts []string
	switch runtime.GOOS {
	case "linux":
		parts = []string{
			safeRead("/etc/machine-id"),
			safeRead("/var/lib/dbus/machine-id"),
			run("cat", "/sys/class/dmi/id/product_uuid"),
		}
	case "windows":
		parts = []string{
			run("wmic", "csproduct", "get", "uuid"),
			run("powershell", "-command", "Get-WmiObject Win32_ComputerSystemProduct | Select-Object -ExpandProperty UUID"),
		}
	case "darwin":
		parts = []string{
			run("sh", "-c", `ioreg -rd1 -c IOPlatformExpertDevice | awk '/IOPlatformUUID/ { print $3; }'`),
		}
	default:
		parts = []string{"unknown-os"}
	}

	fingerprint := strings.Join(filterEmpty(parts), "|")
	hash := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(hash[:])
}

func safeRead(path string) string {
	out, err := exec.Command("cat", path).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func filterEmpty(input []string) []string {
	var out []string
	for _, s := range input {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}