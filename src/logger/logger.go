// Package logger provides a simple, thread-safe, color-coded logging system
// for terminal applications. It supports different log levels, category tags, and optionally reads log level visibility from a `settings.yaml` config file.
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
    mu sync.Mutex

    // Style for timestamps
    timestampStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#676767")) // Dim Grey

    // Centralized style lookup for log levels and types
    styleMap = map[string]lipgloss.Style{
        // Log levels
        "INFO":    lipgloss.NewStyle().Foreground(lipgloss.Color("#00D8A7")), // Pristine Oceanic
        "DEBUG":   lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7DFF")), // Periwinkle
        "ERROR":   lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")), // Fusion Red
        "WARNING": lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")), // Orange

        // Log types
        "HEARTBEAT": lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC0CB")), // Pink
        "MAIN":      lipgloss.NewStyle().Foreground(lipgloss.Color("#D8CB00")), // Groovy Lemon Pie
        "NODEID":    lipgloss.NewStyle().Foreground(lipgloss.Color("#C7F5C1")), // Tea Green
        "UPNP":      lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")), // Deep Sky Blue
        "BOOTSTRAP": lipgloss.NewStyle().Foreground(lipgloss.Color("#DA70D6")), // Orchid
        "UPDATER":   lipgloss.NewStyle().Foreground(lipgloss.Color("#F5E050")), // Lemon Yellow
        "SETTINGS":  lipgloss.NewStyle().Foreground(lipgloss.Color("#40E0D0")), // Turquoise
    }
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

    upperLevel = strings.ToUpper(level)
    levelStyled := upperLevel
    if style, ok := styleMap[upperLevel]; ok {
        levelStyled = style.Render(upperLevel)
    }

    upperTyp := strings.ToUpper(typ)
    typStyled := upperTyp
    if style, ok := styleMap[upperTyp]; ok {
        typStyled = style.Render(upperTyp)
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