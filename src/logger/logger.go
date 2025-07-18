// Package logger provides a simple, thread-safe, color-coded logging system
// for terminal applications. It supports different log levels, category tags,
// and optionally reads log level visibility from a `settings.yaml` config file.
package logger

import (
    "fmt"
    "strings"
    "sync"
    "time"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
    "github.com/charmbracelet/lipgloss"
)

var (
    // logs stores the most recent log entries (up to 500).
    logs []string

    // mu ensures thread-safe access to the logs slice.
    mu   sync.Mutex

    // Styles for different log levels and types
    styleInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D8A7")) // Pristine Oceanic
    styleDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7DFF")) // Periwinkle
    styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")) // Fusion Red
    styleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange
    styleHeartbeat = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC0CB")) // Pink
    styleMain = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8CB00")) // Groovy Lemon Pie
    styleNodeid = lipgloss.NewStyle().Foreground(lipgloss.Color("#C7F5C1")) // Tea Green

    // Style for timestamps
    timestampStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#676767")) // Dim Grey
)

// logLevels defines whether a specific log level is enabled.
// These can be overridden via settings.yaml if present.
var logLevels = map[string]bool{
	"debug":   false,
	"info":    true,
	"warning": true,
	"caution": true,
	"error":   true,
}

// init reads the logger configuration from settings.yaml (if found in the same directory as the executable).
// It updates the logLevels map accordingly. If the file or keys are missing, defaults are used.
func init() {
    exePath, err := os.Executable()
    if err != nil {
        return
    }
    exeDir := filepath.Dir(exePath)
    configFile := filepath.Join(exeDir, "settings.yaml")

    raw, err := os.ReadFile(configFile)
    if err != nil {
        return // Fail silently and stick to defaults
    }

    var parsed map[string]interface{}
    if err := yaml.Unmarshal(raw, &parsed); err != nil {
        return
    }

    if loggerConfig, ok := parsed["logger"].(map[string]interface{}); ok {
        for level := range logLevels {
            if val, ok := loggerConfig[level].(bool); ok {
                logLevels[level] = val
            }
        }
    }
}

// Log adds a new log entry with the given level, type, and message.
// Supported levels include: INFO, DEBUG, WARNING, ERROR.
// Types are user-defined and help categorize logs (e.g., MAIN, NODEID, HEARTBEAT).
// Entries are color-styled for improved terminal readability.
func Log(level string, typ string, message string) {
    mu.Lock()
    defer mu.Unlock()

    upperLevel := strings.ToUpper(level)

    // Check if the level is enabled in settings.yaml
    if !isLevelEnabled(upperLevel) {
        return
    }

    // Choose color style for log level
    var levelStyled string
    switch upperLevel {
    case "INFO":
        levelStyled = styleInfo.Render(upperLevel)
    case "DEBUG":
        levelStyled = styleDebug.Render(upperLevel)
    case "ERROR":
        levelStyled = styleError.Render(upperLevel)
    case "WARNING":
        levelStyled = styleWarning.Render(upperLevel)
    default:
        levelStyled = upperLevel
    }

    // Choose color style for log type/category
    var typStyled string
    upperTyp := strings.ToUpper(typ)

    switch upperTyp {
    case "HEARTBEAT":
        typStyled = styleHeartbeat.Render(upperTyp)
    case "MAIN":
        typStyled = styleMain.Render(upperTyp)
    case "NODEID":
        typStyled = styleNodeid.Render(upperTyp)
    default:
        typStyled = upperTyp
    }

    // Construct formatted log entry
    timeStyled := timestampStyled.Render(time.Now().Format("15:04:05"))
    entry := fmt.Sprintf("%s | %-6s | %-8s | %s", timeStyled, levelStyled, typStyled, message)

    logs = append(logs, entry)

    // Trim to the latest 500 logs
    if len(logs) > 500 {
        logs = logs[1:]
    }
}

// GetLogs returns a copy of the current log history (up to 500 entries).
func GetLogs() []string {
    mu.Lock()
    defer mu.Unlock()
    return append([]string{}, logs...)
}

// isLevelEnabled returns true if the given level is enabled in the config or defaults.
func isLevelEnabled(level string) bool {
	return logLevels[strings.ToLower(level)]
}