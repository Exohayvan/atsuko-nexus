package logger

import (
    "fmt"
    "strings"
    "sync"
    "time"
	"os"

	"gopkg.in/yaml.v3"
    "github.com/charmbracelet/lipgloss"
)

var (
    logs []string
    mu   sync.Mutex

    styleInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D8A7")) // Pristine Oceanic
    styleDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D7DFF")) // Periwinkle
    styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")) // Fusion Red
    styleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange

    styleHeartbeat = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC0CB")) // Pink
    styleMain = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8CB00")) //Groovy Lemon Pie
    styleNodeid = lipgloss.NewStyle().Foreground(lipgloss.Color("#C7F5C1")) //Tea Green

    timestampStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#676767")) // Dim Grey
)

var logLevels = map[string]bool{
	"debug":   false,
	"info":    true,
	"warning": true,
	"caution": true,
	"error":   true,
}

func init() {
	raw, err := os.ReadFile("./settings.yaml")
	if err != nil {
		// Fallback to defaults if file unreadable
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
	}
}

func Log(level string, typ string, message string) {
    mu.Lock()
    defer mu.Unlock()

    upperLevel := strings.ToUpper(level)

    // Check if the level is enabled in settings.yaml
    if !isLevelEnabled(upperLevel) {
        return
    }

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

    timeStyled := timestampStyled.Render(time.Now().Format("15:04:05"))
    entry := fmt.Sprintf("%s | %-6s | %-8s | %s", timeStyled, levelStyled, typStyled, message)

    logs = append(logs, entry)

    if len(logs) > 500 {
        logs = logs[1:]
    }
}

// GetLogs returns a copy of the log list
func GetLogs() []string {
    mu.Lock()
    defer mu.Unlock()
    return append([]string{}, logs...)
}

// isLevelEnabled checks config for that level
func isLevelEnabled(level string) bool {
	return logLevels[strings.ToLower(level)]
}