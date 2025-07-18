// Package nodeid provides a platform-independent method for generating a unique and consistent Node ID based on system-specific identifiers.
// This ID is hashed to ensure privacy and uniformity across platforms.
package nodeid

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"runtime"
	"strings"

	"atsuko-nexus/src/logger"
)

// GetNodeID generates a unique, deterministic identifier for the current machine.
// It collects OS-specific identifiers, joins them into a fingerprint, and returns a SHA-256 hash.
// This ID can be used to uniquely identify a node in a distributed system.
func GetNodeID() string {
	logger.Log("DEBUG", "NODEID", "Getting NodeID...")
	var parts []string

	// Select fingerprinting strategy based on the operating system
	logger.Log("DEBUG", "NODEID", "Detected Runtime: "+runtime.GOOS)
	switch runtime.GOOS {
	case "linux":
		// Try multiple machine-specific files for more consistent identity
		part1 := safeRead("/etc/machine-id")
		part2 := safeRead("/var/lib/dbus/machine-id")
		part3 := run("cat", "/sys/class/dmi/id/product_uuid")
		parts = []string{part1, part2, part3}

		logger.Log("DEBUG", "NODEID", "/etc/machine-id: "+part1)
		logger.Log("DEBUG", "NODEID", "/var/lib/dbus/machine-id: "+part2)
		logger.Log("DEBUG", "NODEID", "/sys/class/dmi/id/product_uuid: "+part3)

	case "windows":
		// Use WMIC and PowerShell to gather system UUIDs
		part1 := run("wmic", "csproduct", "get", "uuid")
		part2 := run("powershell", "-command", "Get-WmiObject Win32_ComputerSystemProduct | Select-Object -ExpandProperty UUID")
		parts = []string{part1, part2}

		logger.Log("DEBUG", "NODEID", "wmic UUID: "+part1)
		logger.Log("DEBUG", "NODEID", "powershell UUID: "+part2)

	case "darwin":
		// macOS: Use ioreg to get the platform UUID
		part1 := run("sh", "-c", `ioreg -rd1 -c IOPlatformExpertDevice | awk '/IOPlatformUUID/ { print $3; }'`)
		parts = []string{part1}

		logger.Log("DEBUG", "NODEID", "IOPlatformUUID: "+part1)

	default:
		// Fallback for unknown OS
		logger.Log("WARN", "NODEID", "Unknown OS: using fallback")
		parts = []string{"unknown-os"}
	}

	// Join all valid parts into a fingerprint
	fingerprint := strings.Join(filterEmpty(parts), "|")
	logger.Log("DEBUG", "NODEID", "Final Fingerprint: "+fingerprint)

	// Hash the fingerprint using SHA-256
	hash := sha256.Sum256([]byte(fingerprint))
	finalID := hex.EncodeToString(hash[:])
	logger.Log("DEBUG", "NODEID", "Final Hashed NodeID: "+finalID)

	return finalID
}

// safeRead attempts to read a file from disk and trims whitespace.
// If the file cannot be read, it returns an empty string and logs the error.
func safeRead(path string) string {
	out, err := exec.Command("cat", path).Output()
	if err != nil {
		logger.Log("DEBUG", "NODEID", "Failed to read "+path+": "+err.Error())
		return ""
	}
	result := strings.TrimSpace(string(out))
	logger.Log("DEBUG", "NODEID", "safeRead("+path+"): "+result)
	return result
}

// run executes a system command with arguments and returns trimmed output.
// If the command fails, it returns an empty string and logs the error.
func run(name string, args ...string) string {
	cmd := name + " " + strings.Join(args, " ")
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		logger.Log("DEBUG", "NODEID", "Failed to run "+cmd+": "+err.Error())
		return ""
	}
	result := strings.TrimSpace(string(out))
	logger.Log("DEBUG", "NODEID", "run("+cmd+"): "+result)
	return result
}

// filterEmpty removes empty or whitespace-only strings from the input slice.
func filterEmpty(input []string) []string {
	var out []string
	for _, s := range input {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
