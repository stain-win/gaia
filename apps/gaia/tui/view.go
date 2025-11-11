package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"strings"
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
	case accessManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.accessMenu.View())
	case certManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.certMenu.View())
	case policyManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.policyMenu.View())
	case createCerts:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.certForm.View())
	case registerClient:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.registerClientFormModel.View())
	case listRecords:
		screenView = m.inspector.View()
	case listPolicies:
		screenView = m.renderPolicyListView()
	case viewPolicy:
		if m.selectedPolicy != nil {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.selectedPolicy.viewport.View())
		} else {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, "No policy selected")
		}
	case selectPolicyClient:
		screenView = m.renderClientSelectorView()
	case createPolicy, editPolicy:
		if m.policyEditorModel != nil && m.policyEditorModel.form != nil {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.policyEditorModel.form.View())
		} else {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, "Loading...")
		}
	case deletePolicy:
		if m.policyDeleteModel != nil && m.policyDeleteModel.form != nil {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.policyDeleteModel.form.View())
		} else {
			screenView = lipgloss.JoinVertical(lipgloss.Center, logo, "Loading...")
		}
	case policyExport:
		screenView = m.renderExportOptionsView()
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

func (m *model) renderPolicyListView() string {
	if len(m.policies) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(2).
			Align(lipgloss.Center)

		return lipgloss.JoinVertical(
			lipgloss.Center,
			emptyStyle.Render("No policies found"),
			emptyStyle.Render("Press 'b' to go back"),
		)
	}

	items := make([]list.Item, len(m.policies))
	for i, p := range m.policies {
		items[i] = p
	}

	l := list.New(items, list.NewDefaultDelegate(), m.width-4, m.height-10)
	l.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Authorization Policies")
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return lipgloss.JoinVertical(lipgloss.Center, l.View())
}

func (m *model) renderClientSelectorView() string {
	if len(m.clients) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(2).
			Align(lipgloss.Center)

		return lipgloss.JoinVertical(
			lipgloss.Center,
			emptyStyle.Render("No clients found"),
			emptyStyle.Render("Press 'b' to go back"),
		)
	}

	// Create a list of clients
	items := make([]list.Item, len(m.clients))
	for i, clientName := range m.clients {
		items[i] = menuItem{title: clientName, desc: "Select to manage policy"}
	}

	l := list.New(items, list.NewDefaultDelegate(), m.width-4, m.height-10)
	l.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Select Client")
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return lipgloss.JoinVertical(lipgloss.Center, l.View())
}

func (m *model) renderExportOptionsView() string {
	if m.selectedPolicy == nil {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			"Loading policy...",
		)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		MarginBottom(1)

	optionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00CED1")).
		MarginLeft(2)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true).
		MarginTop(2)

	title := titleStyle.Render(fmt.Sprintf("Export Policy: %s", m.selectedPolicy.policy.ClientName))

	options := []string{
		optionStyle.Render("Press 'j' or '1' - Export as JSON"),
		optionStyle.Render("Press 'y' or '2' - Export as YAML"),
		"",
		helpStyle.Render("Press 'esc' to cancel"),
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		strings.Join(options, "\n"),
	)
}
