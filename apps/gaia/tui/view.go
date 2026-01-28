package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// renderStatusBar creates a clean status bar with connection state and messages
func (m *model) renderStatusBar() string {
	// Left section: Status badge
	var statusBadge string
	switch m.daemonStatus {
	case DaemonStatusUnlocked:
		statusBadge = statusBadgeUnlocked.Render("● UNLOCKED")
	case DaemonStatusLocked:
		statusBadge = statusBadgeLocked.Render("◐ LOCKED")
	case DaemonStatusOffline, DaemonStatusStopped, DaemonStatusStarting:
		statusBadge = statusBadgeOffline.Render("○ OFFLINE")
	default:
		statusBadge = statusBadgeOffline.Render("○ UNKNOWN")
	}

	// Right section: Version info
	versionInfo := helpDescStyle.Render("Gaia Secret Manager")

	// Calculate widths
	badgeWidth := lipgloss.Width(statusBadge)
	versionWidth := lipgloss.Width(versionInfo)
	paddingWidth := m.width - badgeWidth - versionWidth - 4

	if paddingWidth < 0 {
		paddingWidth = 1
	}

	// Build status bar
	statusBar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		statusBadge,
		strings.Repeat(" ", paddingWidth),
		versionInfo,
	)

	return statusBarStyle.Width(m.width).Render(statusBar)
}

// renderMessageBar shows status messages below the status bar
func (m *model) renderMessageBar() string {
	if m.statusMessage == "" {
		return ""
	}

	var msgStyle lipgloss.Style
	switch {
	case strings.HasPrefix(m.statusMessage, "✓"):
		msgStyle = successMessageStyle
	case strings.HasPrefix(m.statusMessage, "❌"), strings.HasPrefix(m.statusMessage, "⚠️"):
		msgStyle = warningMessageStyle
	default:
		msgStyle = messageStyle
	}

	msg := msgStyle.Render(m.statusMessage)
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Padding(0, 1).
		Render(msg)
}

// renderHelpBar creates a help bar with keyboard shortcuts for the current screen
func (m *model) renderHelpBar() string {
	var keys []string

	switch m.activeScreen {
	case mainMenu:
		keys = []string{
			helpKeyStyle.Render("↑/↓") + " " + helpDescStyle.Render("navigate"),
			helpKeyStyle.Render("enter") + " " + helpDescStyle.Render("select"),
			helpKeyStyle.Render("q") + " " + helpDescStyle.Render("quit"),
		}
	case dataManagement, accessManagement, certManagement, policyManagement:
		keys = []string{
			helpKeyStyle.Render("↑/↓") + " " + helpDescStyle.Render("navigate"),
			helpKeyStyle.Render("enter") + " " + helpDescStyle.Render("select"),
			helpKeyStyle.Render("b/esc") + " " + helpDescStyle.Render("back"),
		}
	case addRecord, createCerts, registerClient, createPolicy, editPolicy:
		keys = []string{
			helpKeyStyle.Render("tab") + " " + helpDescStyle.Render("next field"),
			helpKeyStyle.Render("enter") + " " + helpDescStyle.Render("submit"),
			helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("cancel"),
		}
	case listRecords, listPolicies, viewPolicy:
		keys = []string{
			helpKeyStyle.Render("↑/↓") + " " + helpDescStyle.Render("scroll"),
			helpKeyStyle.Render("/") + " " + helpDescStyle.Render("filter"),
			helpKeyStyle.Render("b/esc") + " " + helpDescStyle.Render("back"),
		}
	case unlockScreen:
		keys = []string{
			helpKeyStyle.Render("enter") + " " + helpDescStyle.Render("unlock"),
			helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("cancel"),
		}
	case deletePolicy:
		keys = []string{
			helpKeyStyle.Render("y") + " " + helpDescStyle.Render("confirm"),
			helpKeyStyle.Render("n/esc") + " " + helpDescStyle.Render("cancel"),
		}
	case policyExport:
		keys = []string{
			helpKeyStyle.Render("j") + " " + helpDescStyle.Render("export JSON"),
			helpKeyStyle.Render("y") + " " + helpDescStyle.Render("export YAML"),
			helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("cancel"),
		}
	default:
		keys = []string{
			helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("back"),
			helpKeyStyle.Render("q") + " " + helpDescStyle.Render("quit"),
		}
	}

	helpText := strings.Join(keys, "  │  ")
	return helpBarStyle.Width(m.width).Align(lipgloss.Center).Render(helpText)
}

func (m *model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Logo with styling
	logo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6A5ACD")).
		Align(lipgloss.Center).
		Render(gaiaLogo)

	// Build screen content based on active screen
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
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.renderExportOptionsView())
	case unlockScreen:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, m.unlockFormModel.View())
	}

	// Layout:
	// ┌─────────────────────────────────────┐
	// │ [STATUS BAR]                        │
	// ├─────────────────────────────────────┤
	// │ [MESSAGE BAR - if present]          │
	// ├─────────────────────────────────────┤
	// │                                     │
	// │         [MAIN CONTENT]              │
	// │         (centered)                  │
	// │                                     │
	// ├─────────────────────────────────────┤
	// │ [HELP BAR]                          │
	// └─────────────────────────────────────┘

	statusBar := m.renderStatusBar()
	messageBar := m.renderMessageBar()
	helpBar := m.renderHelpBar()

	// Calculate content height
	statusHeight := 1
	helpHeight := 1
	messageHeight := 0
	if messageBar != "" {
		messageHeight = 1
	}

	contentHeight := m.height - statusHeight - helpHeight - messageHeight - 2

	// Center the main content
	centeredContent := lipgloss.Place(
		m.width,
		contentHeight,
		lipgloss.Center,
		lipgloss.Center,
		screenView,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#383838")),
	)

	// Build final layout
	var sections []string
	sections = append(sections, statusBar)
	if messageBar != "" {
		sections = append(sections, messageBar)
	}
	sections = append(sections, centeredContent)
	sections = append(sections, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *model) renderPolicyListView() string {
	if len(m.policies) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(colorTextMuted).
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
	l.Title = titleStyle.Render("Authorization Policies")
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return lipgloss.JoinVertical(lipgloss.Center, l.View())
}

func (m *model) renderClientSelectorView() string {
	if len(m.clients) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(colorTextMuted).
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
	l.Title = titleStyle.Render("Select Client")
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return lipgloss.JoinVertical(lipgloss.Center, l.View())
}

func (m *model) renderExportOptionsView() string {
	if m.selectedPolicy == nil {
		return "Loading policy..."
	}

	boxStyle := contentBoxStyle.
		Width(50).
		Align(lipgloss.Center)

	title := titleStyle.Render(fmt.Sprintf("Export Policy: %s", m.selectedPolicy.policy.ClientName))

	optionStyle := lipgloss.NewStyle().
		Foreground(colorSecondary).
		Padding(0, 2)

	options := []string{
		"",
		optionStyle.Render("[j] Export as JSON"),
		optionStyle.Render("[y] Export as YAML"),
		"",
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		strings.Join(options, "\n"),
	)

	return boxStyle.Render(content)
}
