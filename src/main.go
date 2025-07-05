package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"atsuko-nexus/src/logger"
)

type model struct {
	viewport viewport.Model
	ready    bool
}

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-2)
			m.viewport.Style = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
		return m, nil

	case tickMsg:
		wasAtBottom := m.viewport.AtBottom()

		logger.Log("INFO", "heartbeat", "Node still alive")
		m.viewport.SetContent(strings.Join(logger.GetLogs(), "\n"))

		if wasAtBottom {
			m.viewport.GotoBottom()
		}

		return m, tick()

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
	header := lipgloss.NewStyle().Bold(true).Render("💠 Atsuko Nexus - Headless Mode (press 'q' to quit)")
	return header + "\n" + m.viewport.View()
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if err := p.Start(); err != nil {
		panic(err)
	}
}