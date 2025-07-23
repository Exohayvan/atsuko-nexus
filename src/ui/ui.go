// Package ui provides the interactive terminal user interface (TUI) for Atsuko Nexus.
// It displays live system metrics, logs, and general status information using the Bubble Tea framework.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/settings"
	"atsuko-nexus/src/version"
	"atsuko-nexus/src/p2p"
)

var (
	startTime     = time.Now() // Used to calculate uptime
	nodeID        string // The Node ID shown in the UI
	lastBytesSent uint64 // Used to track network upload delta
	lastBytesRecv uint64 // Used to track network download delta
	lastNetTime   time.Time // Last time network was sampled
)

// model defines the Bubble Tea view model with viewport support.
type model struct {
	viewport viewport.Model
	ready    bool
}

type tickMsg struct{}      // Message used to trigger log refresh
type heartbeatMsg struct{} // Message used to trigger system metric heartbeat

// getUptime returns a human-readable duration string since app launch.
func getUptime() string {
	return time.Since(startTime).Round(time.Second).String()
}

// formatBytes converts a byte count into a human-readable string with units.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// tick returns a Bubble Tea command that sends a tickMsg at the configured interval.
func tick() tea.Cmd {
	refreshSec := settings.Get("ui.panel_refresh_time")
	refreshDur := time.Second

	if sec, ok := refreshSec.(int); ok && sec > 0 {
		refreshDur = time.Duration(sec) * time.Second
	} else if fsec, ok := refreshSec.(float64); ok && fsec > 0 {
		refreshDur = time.Duration(fsec) * time.Second
	}

	return tea.Tick(refreshDur, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// heartbeatTick returns a Bubble Tea command that sends a heartbeatMsg on interval.
func heartbeatTick() tea.Cmd {
	interval := settings.Get("metrics.heartbeat_interval")
	intervalDur := 120 * time.Second

	if sec, ok := interval.(int); ok && sec > 0 {
		intervalDur = time.Duration(sec) * time.Second
	} else if fsec, ok := interval.(float64); ok && fsec > 0 {
		intervalDur = time.Duration(fsec) * time.Second
	}

	return tea.Tick(intervalDur, func(t time.Time) tea.Msg {
		return heartbeatMsg{}
	})
}

// Init sets up the Bubble Tea program to run both the tick and heartbeat loops.
func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), heartbeatTick())
}

// Update handles user interaction, window size changes, tick/heartbeat messages, and viewport updates.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if !m.ready {
			logger.Log("DEBUG", "UI", fmt.Sprintf("Initial window size: %dx%d", msg.Width, msg.Height))
			m.viewport = viewport.New(msg.Width, msg.Height-3)
			m.viewport.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
			m.ready = true
		} else {
			logger.Log("DEBUG", "UI", fmt.Sprintf("Window resized to: %dx%d", msg.Width, msg.Height))
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3
		}
		return m, nil

	case tickMsg:
		wasAtBottom := m.viewport.AtBottom()
		m.viewport.SetContent(strings.Join(logger.GetLogs(), "\n"))
		if wasAtBottom {
			m.viewport.GotoBottom()
		}
		return m, tick()

	case heartbeatMsg:
		logMsg := "Node still alive"
		logger.Log("DEBUG", "heartbeat", "heartbeatMsg received, collecting metrics...")

		if settings.Get("metrics.enable_metrics") == true {
			parts := []string{}

			if settings.Get("metrics.cpu_monitoring") == true {
				if usage, _ := cpu.Percent(0, false); len(usage) > 0 {
					logger.Log("DEBUG", "heartbeat", fmt.Sprintf("CPU usage: %.1f%%", usage[0]))
					parts = append(parts, fmt.Sprintf("CPU: %.1f%%", usage[0]))
				}
			}

			if settings.Get("metrics.ram_monitoring") == true {
				if vmStat, _ := mem.VirtualMemory(); vmStat != nil {
					logger.Log("DEBUG", "heartbeat", fmt.Sprintf("RAM usage: %.1f%% (%s/%s)",
						vmStat.UsedPercent,
						formatBytes(vmStat.Used),
						formatBytes(vmStat.Total)))
					parts = append(parts, fmt.Sprintf("RAM: %.1f%% (%s/%s)",
						vmStat.UsedPercent,
						formatBytes(vmStat.Used),
						formatBytes(vmStat.Total),
					))
				}
			}

			if settings.Get("metrics.net_traffic_monitoring") == true {
				if ioStat, _ := net.IOCounters(false); len(ioStat) > 0 {
					now := time.Now()
					elapsed := now.Sub(lastNetTime).Seconds()

					deltaSent := float64(ioStat[0].BytesSent - lastBytesSent)
					deltaRecv := float64(ioStat[0].BytesRecv - lastBytesRecv)

					upRate := deltaSent / elapsed
					downRate := deltaRecv / elapsed

					logger.Log("DEBUG", "heartbeat", fmt.Sprintf("Net ↑ %s/s ↓ %s/s",
						formatBytes(uint64(upRate)),
						formatBytes(uint64(downRate))))

					lastBytesSent = ioStat[0].BytesSent
					lastBytesRecv = ioStat[0].BytesRecv
					lastNetTime = now

					parts = append(parts, fmt.Sprintf("Net: ↑ %s/s ↓ %s/s",
						formatBytes(uint64(upRate)),
						formatBytes(uint64(downRate)),
					))
				}
			}

			if len(parts) > 0 {
				logMsg = strings.Join(parts, " | ")
			}
		}

		logger.Log("INFO", "heartbeat", logMsg)
		return m, heartbeatTick()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the main TUI panel: header, status line, help line, and log viewport.
func (m model) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Render("💠 Atsuko Nexus 💠")

	status := lipgloss.NewStyle().
		Faint(true).
		Render(fmt.Sprintf("Version: %s | Uptime: %s | Node ID: %s | Peers: %d", version.Current, getUptime(), nodeID, p2p.CountActivePeers()))

	help := lipgloss.NewStyle().
		Italic(true).
		Faint(true).
		Render("Press 's' = settings | 'q' = quit")

	return header + "\n" + status + "\n" + help + "\n" + m.viewport.View()
}

// Start launches the user interface, initializing network counters and running the TUI.
func Start(id string) {
	logger.Log("DEBUG", "UI", "UI Start() called.")
	nodeID = id
	lastNetTime = time.Now()

	logger.Log("DEBUG", "UI", "Initializing network counters...")
	if counters, _ := net.IOCounters(false); len(counters) > 0 {
		lastBytesSent = counters[0].BytesSent
		lastBytesRecv = counters[0].BytesRecv
		logger.Log("DEBUG", "UI", fmt.Sprintf("Initial Bytes Sent: %d, Bytes Recv: %d", lastBytesSent, lastBytesRecv))
	}

	logger.Log("INFO", "UI", "Launching TUI...")
	p := tea.NewProgram(model{}, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		logger.Log("ERROR", "UI", fmt.Sprintf("TUI crashed: %v", err))
		panic(err)
	}
	logger.Log("INFO", "UI", "TUI closed gracefully.")
}
