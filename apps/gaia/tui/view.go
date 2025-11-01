package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m *model) statusView() string {
	// Create a status indicator with visual icons
	var statusIcon, statusColor string
	var statusBg lipgloss.Color

	switch m.daemonStatus {
	case DaemonStatusUnlocked:
		statusIcon = "🔓"
		statusColor = "#00FF00" // Green
		statusBg = "#003300"
	case DaemonStatusLocked:
		statusIcon = "🔒"
		statusColor = "#FFA500" // Orange
		statusBg = "#332200"
	case DaemonStatusOffline, DaemonStatusStopped, DaemonStatusStarting:
		statusIcon = "⚠️"
		statusColor = "#FF0000" // Red
		statusBg = "#330000"
	default:
		statusIcon = "❓"
		statusColor = "#888888" // Gray
		statusBg = "#222222"
	}

	statusText := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor)).
		Bold(true).
		Render(statusIcon + " " + m.daemonStatus)

	// Add a status message if present
	var fullStatus string
	if m.statusMessage != "" {
		msgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Italic(true)
		fullStatus = statusText + "  |  " + msgStyle.Render(m.statusMessage)
	} else {
		fullStatus = statusText
	}

	return lipgloss.NewStyle().
		Background(statusBg).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center).
		Border(lipgloss.DoubleBorder(), false, false, true, false).
		BorderForeground(statusBg).
		Render(fullStatus)
}

func (m *model) View() string {
	logo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6A5ACD")).
		Align(lipgloss.Center).
		Render(gaiaLogo)

	var screenView string
	switch m.activeScreen {
	case mainMenu:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.mainMenu.View())
	case dataManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.dataMenu.View())
	case addRecord:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.addRecordFormModel.View())
	case certManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.certMenu.View())
	case createCerts:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.certForm.View())
	case registerClient:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.registerClientFormModel.View())
	case listRecords:
		screenView = m.inspector.View()
	case unlockScreen:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.unlockFormModel.View())
	}

	// Show the status bar at the top
	statusBar := m.statusView()

	content := lipgloss.JoinVertical(lipgloss.Center, statusBar, screenView)
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}),
	)
}
