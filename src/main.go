package main

import (
	"strings"
	"time"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
)

var (
	version   = "v0.1.0-alpha"
	startTime = time.Now()
	nodeID    = nodeid.GetNodeID()
)

type model struct {
	viewport viewport.Model
	ready    bool
}

type tickMsg struct{}
type heartbeatMsg struct{}

func getUptime() string {
	return time.Since(startTime).Round(time.Second).String()
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func heartbeatTick() tea.Cmd {
	return tea.Tick(120*time.Second, func(t time.Time) tea.Msg {
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
		logger.Log("INFO", "heartbeat", "Node still alive")
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
		Render("💠 Atsuko Nexus - Headless Mode")

	status := lipgloss.NewStyle().
		Faint(true).
		Render(fmt.Sprintf("Version: %s | Uptime: %s | Node ID: %s", version, getUptime(), nodeID))

	help := lipgloss.NewStyle().
		Italic(true).
		Faint(true).
		Render("Press 's' = settings | 'q' = quit")

	return header + "\n" + status + "\n" + help + "\n" + m.viewport.View()
}

func main() {
	logger.Log("INFO", "MAIN", "Script started with ID: "+nodeID)
	p := tea.NewProgram(model{}, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
