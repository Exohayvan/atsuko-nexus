package main

import (
	"strings"
	"time"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/mem"
    "github.com/shirou/gopsutil/v3/net"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
    "atsuko-nexus/src/settings"
)

var (
	version   = "v0.1.0-alpha"
	startTime = time.Now()
	nodeID    = nodeid.GetNodeID()
    peers = "0"
    lastBytesSent uint64
    lastBytesRecv uint64
    lastNetTime time.Time
)

type model struct {
	viewport viewport.Model
	ready    bool
}

type tickMsg struct{}
type heartbeatMsg struct {}

func getUptime() string {
	return time.Since(startTime).Round(time.Second).String()
}

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

func tick() tea.Cmd {
    refreshSec := settings.Get("ui.panel_refresh_time")
    refreshDur := time.Second // default fallback

    if sec, ok := refreshSec.(int); ok && sec > 0 {
        refreshDur = time.Duration(sec) * time.Second
    } else if fsec, ok := refreshSec.(float64); ok && fsec > 0 {
        refreshDur = time.Duration(fsec) * time.Second
    }

    return tea.Tick(refreshDur, func(t time.Time) tea.Msg {
        return tickMsg{}
    })
}

func heartbeatTick() tea.Cmd {
    interval := settings.Get("metrics.heartbeat_interval")
    intervalDur := 120 * time.Second // default fallback

    if sec, ok := interval.(int); ok && sec > 0 {
        intervalDur = time.Duration(sec) * time.Second
    } else if fsec, ok := interval.(float64); ok && fsec > 0 {
        intervalDur = time.Duration(fsec) * time.Second
    }

    return tea.Tick(intervalDur, func(t time.Time) tea.Msg {
        return heartbeatMsg{}
    })
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tick(),          // UI refresh every second
		heartbeatTick(), // Heartbeat log every 120 seconds
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-3)
			m.viewport.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
			m.ready = true
		} else {
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
    // Default message
    logMsg := "Node still alive"

    // Check if metrics are enabled
    if settings.Get("metrics.enable_metrics") == true {
        parts := []string{}

        // CPU usage
        if settings.Get("metrics.cpu_monitoring") == true {
            if usage, _ := cpu.Percent(0, false); len(usage) > 0 {
                parts = append(parts, fmt.Sprintf("CPU: %.1f%%", usage[0]))
            }
        }

        // RAM usage
        if settings.Get("metrics.ram_monitoring") == true {
            if vmStat, _ := mem.VirtualMemory(); vmStat != nil {
                parts = append(parts, fmt.Sprintf("RAM: %.1f%% (%s/%s)",
                    vmStat.UsedPercent,
                    formatBytes(vmStat.Used),
                    formatBytes(vmStat.Total),
                ))
            }
        }

        // Network usage
        if settings.Get("metrics.net_traffic_monitoring") == true {
            if ioStat, _ := net.IOCounters(false); len(ioStat) > 0 {
                now := time.Now()
                elapsed := now.Sub(lastNetTime).Seconds()

                deltaSent := float64(ioStat[0].BytesSent - lastBytesSent)
                deltaRecv := float64(ioStat[0].BytesRecv - lastBytesRecv)

                upRate := deltaSent / elapsed
                downRate := deltaRecv / elapsed

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

func (m model) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Render("💠 Atsuko Nexus 💠")

	status := lipgloss.NewStyle().
		Faint(true).
		Render(fmt.Sprintf("Version: %s | Uptime: %s | Node ID: %s | Peers: %s", version, getUptime(), nodeID, peers))

	help := lipgloss.NewStyle().
		Italic(true).
		Faint(true).
		Render("Press 's' = settings | 'q' = quit")

	return header + "\n" + status + "\n" + help + "\n" + m.viewport.View()
}

func main() {
    lastNetTime = time.Now()
    if counters, _ := net.IOCounters(false); len(counters) > 0 {
        lastBytesSent = counters[0].BytesSent
        lastBytesRecv = counters[0].BytesRecv
    }

    logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)
    p := tea.NewProgram(model{}, tea.WithAltScreen(), tea.WithMouseCellMotion())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
