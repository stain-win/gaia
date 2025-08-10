package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m *model) View() string {
	logo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6A5ACD")).
		Align(lipgloss.Center).
		Render(gaiaLogo)

	var screenView string
	switch m.activeScreen {
	case mainMenu:
		screenView = m.mainMenu.View()
	case dataManagement:
		screenView = m.dataMenu.View()
	case addRecord:
		screenView = m.addRecordFormModel.View()
	case certManagement:
		screenView = m.certMenu.View()
	case createCerts:
		screenView = m.certForm.View()
	case registerClient:
		screenView = m.registerClientFormModel.View()
	case listRecords:
		screenView = m.viewListRecords()
	}

	content := lipgloss.JoinVertical(lipgloss.Center, logo, screenView)
	return appStyle.
		Width(m.width).
		Render(content)
}
