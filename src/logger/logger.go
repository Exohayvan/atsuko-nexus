package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"

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

	timestampStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#676767")) // Dim Grey
)

// Log logs a formatted entry: time | LEVEL | TYPE | message
func Log(level string, typ string, message string) {
	mu.Lock()
	defer mu.Unlock()

	var levelStyled string
	upperLevel := strings.ToUpper(level)

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