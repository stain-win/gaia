package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m *model) statusView() string {
	return statusBarStyle.
		Align(lipgloss.Center).
		Render(m.daemonStatus)
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
	}

	content := lipgloss.JoinVertical(lipgloss.Left, screenView)
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
