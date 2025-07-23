// Package logger provides a simple, thread-safe, color-coded logging system
// for terminal applications. It supports different log levels, category tags,
// and optionally reads log level visibility from a `settings.yaml` config file.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

var (
	// logs stores the most recent log entries (up to 500).
	logs []string
	mu   sync.Mutex

	// Styles
	timestampStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#676767"))
	styleMap        = map[string]lipgloss.Style{
		// Log levels
		"INFO":    lipgloss.NewStyle().Foreground(lipgloss.Color("#00D8A7")),
		"DEBUG":   lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7DFF")),
		"ERROR":   lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")),
		"WARNING": lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")),

		// Log types
		"HEARTBEAT": lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC0CB")),
		"MAIN":      lipgloss.NewStyle().Foreground(lipgloss.Color("#D8CB00")),
		"NODEID":    lipgloss.NewStyle().Foreground(lipgloss.Color("#C7F5C1")),
		"UPNP":      lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")),
		"NEXUS": lipgloss.NewStyle().Foreground(lipgloss.Color("#DA70D6")),
		"UPDATER":   lipgloss.NewStyle().Foreground(lipgloss.Color("#F5E050")),
		"SETTINGS":  lipgloss.NewStyle().Foreground(lipgloss.Color("#40E0D0")),
        "TAPSYNC":  lipgloss.NewStyle().Foreground(lipgloss.Color("#55d3e7")),
        "UI": lipgloss.NewStyle().Foreground(lipgloss.Color("#6937a3")),
	}

	// Config options
	logLevels = map[string]bool{
		"debug":   false,
		"info":    true,
		"warning": true,
		"caution": true,
		"error":   true,
	}

	logToFile     = false
	logFilePath   string
	rotateLogs    = false
	maxLogSizeMB  = 10
	maxLogAgeDays = 7
	logFileHandle *os.File
)

func init() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)
	configFile := filepath.Join(exeDir, "settings.yaml")

	raw, err := os.ReadFile(configFile)
	if err != nil {
		return
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
		if val, ok := loggerConfig["log_to_file"].(bool); ok {
			logToFile = val
		}
		if val, ok := loggerConfig["log_file_path"].(string); ok {
			logFilePath = filepath.Join(exeDir, val)
		}
		if val, ok := loggerConfig["rotate_logs"].(bool); ok {
			rotateLogs = val
		}
		if val, ok := loggerConfig["max_log_size_mb"].(int); ok {
			maxLogSizeMB = val
		}
		if val, ok := loggerConfig["max_log_age_days"].(int); ok {
			maxLogAgeDays = val
		}
	}

	if logToFile && logFilePath != "" {
		setupLogFile()
	}
}

// setupLogFile creates or rotates the log file and deletes old logs by age.
func setupLogFile() {
	_ = os.MkdirAll(filepath.Dir(logFilePath), 0755)

	// Rotate if size exceeds max
	if rotateLogs {
		info, err := os.Stat(logFilePath)
		if err == nil && info.Size() >= int64(maxLogSizeMB)*1024*1024 {
			timestamp := time.Now().Format("20060102-150405")
			rotated := fmt.Sprintf("%s.%s", logFilePath, timestamp)
			_ = os.Rename(logFilePath, rotated)
		}
	}

	// Delete logs older than max age
	if rotateLogs {
		dir := filepath.Dir(logFilePath)
		base := filepath.Base(logFilePath)
		prefix := base + "."
		files, err := os.ReadDir(dir)
		if err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), prefix) {
					path := filepath.Join(dir, file.Name())
					info, err := os.Stat(path)
					if err == nil && time.Since(info.ModTime()).Hours() > float64(maxLogAgeDays*24) {
						_ = os.Remove(path)
					}
				}
			}
		}
	}

	// Open log file
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		logFileHandle = f
	} else {
		logToFile = false // fallback
	}
}

// Log adds a styled log entry to memory and optionally to file.
func Log(level string, typ string, message string) {
	mu.Lock()
	defer mu.Unlock()

	upperLevel := strings.ToUpper(level)
	if !isLevelEnabled(upperLevel) {
		return
	}

	levelStyled := styleMapOrDefault(upperLevel, upperLevel)
	upperTyp := strings.ToUpper(typ)
	typStyled := styleMapOrDefault(upperTyp, upperTyp)
	timeStyled := timestampStyled.Render(time.Now().Format("15:04:05"))
	entry := fmt.Sprintf("%s | %-6s | %-8s | %s", timeStyled, levelStyled, typStyled, message)
	logs = append(logs, entry)

	if len(logs) > 500 {
		logs = logs[1:]
	}

	if logToFile && logFileHandle != nil {
		plain := fmt.Sprintf("%s | %-6s | %-8s | %s\n",
			time.Now().Format("2006-01-02 15:04:05"), upperLevel, upperTyp, message)
		_, _ = logFileHandle.WriteString(plain)
	}
}

func isLevelEnabled(level string) bool {
	return logLevels[strings.ToLower(level)]
}

func styleMapOrDefault(key, fallback string) string {
	if style, ok := styleMap[key]; ok {
		return style.Render(key)
	}
	return fallback
}

func GetLogs() []string {
	mu.Lock()
	defer mu.Unlock()
	return append([]string{}, logs...)
}
