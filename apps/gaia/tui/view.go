package tui

import (
	"fmt"
	"strings"

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

// renderMessageBar shows status messages below the status bar as a styled banner
func (m *model) renderMessageBar() string {
	if m.statusMessage == "" {
		return ""
	}

	var banner lipgloss.Style
	var label string
	switch m.statusMessageType {
	case msgError:
		banner = errorBannerStyle
		label = "✕ ERROR "
	case msgSuccess:
		banner = successBannerStyle
		label = "✓ "
	case msgWarning:
		banner = warningBannerStyle
		label = "⚠ WARNING "
	default:
		banner = infoBannerStyle
		label = ""
	}

	content := label + m.statusMessage
	return banner.Width(m.width).Render(content)
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
	case splashScreen:
		keys = []string{
			helpKeyStyle.Render("any key") + " " + helpDescStyle.Render("skip"),
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

	// Splash screen: full-screen animation, no chrome
	if m.activeScreen == splashScreen && m.splash != nil {
		m.splash.width = m.width
		m.splash.height = m.height
		return m.splash.View()
	}

	// Fixed-width container for content screens (centered by lipgloss.Place below)
	containerWidth := 84
	menuContainer := lipgloss.NewStyle().
		Width(containerWidth).
		Padding(0, 2).
		Align(lipgloss.Left)

	// Compact logo header — same width as the menu container so the block centers properly
	logo := renderCompactHeader(containerWidth)

	// Build screen content based on the active screen
	var screenView string

	switch m.activeScreen {
	case mainMenu:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.mainMenu.View()))
	case dataManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.dataMenu.View()))
	case addRecord:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.addRecordFormModel.View()))
	case accessManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.accessMenu.View()))
	case certManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.certMenu.View()))
	case policyManagement:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.policyMenu.View()))
	case createCerts:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.certForm.View()))
	case registerClient:
		screenView = lipgloss.JoinVertical(lipgloss.Left, logo, menuContainer.Render(m.registerClientFormModel.View()))
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
	default:
		screenView = lipgloss.JoinVertical(lipgloss.Center, logo, fmt.Sprintf("unknown screen: %v", m.activeScreen))
	}

	// Layout:
	// ┌─────────────────────────────────────┐
	// │ [STATUS BAR]                        │
	// ├─────────────────────────────────────┤
	// │ [MESSAGE BAR - if present]          │
	// ├─────────────────────────────────────┤
	// │ [BREADCRUMB]                        │
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
	breadcrumb := m.renderBreadcrumb()
	helpBar := m.renderHelpBar()

	// Calculate content height
	chromeHeight := 2 // status bar + help bar
	if messageBar != "" {
		chromeHeight++
	}
	if breadcrumb != "" {
		chromeHeight++
	}

	contentHeight := m.height - chromeHeight - 2

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
	if breadcrumb != "" {
		sections = append(sections, breadcrumb)
	}
	sections = append(sections, centeredContent)
	sections = append(sections, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderBreadcrumb builds a navigation path for the current screen.
func (m *model) renderBreadcrumb() string {
	var segments []string

	switch m.activeScreen {
	case mainMenu:
		// No breadcrumb on the home screen
		return ""
	case dataManagement:
		segments = []string{"Home", "Data"}
	case addRecord:
		segments = []string{"Home", "Data", "Add Record"}
	case listRecords:
		segments = []string{"Home", "Data", "All Records"}
	case accessManagement:
		segments = []string{"Home", "Access"}
	case certManagement:
		segments = []string{"Home", "Access", "Certificates"}
	case createCerts:
		segments = []string{"Home", "Access", "Certificates", "Create"}
	case registerClient:
		segments = []string{"Home", "Access", "Certificates", "Register Client"}
	case policyManagement:
		segments = []string{"Home", "Access", "Policies"}
	case listPolicies:
		segments = []string{"Home", "Access", "Policies", "List"}
	case viewPolicy:
		segments = []string{"Home", "Access", "Policies", "View"}
	case createPolicy:
		segments = []string{"Home", "Access", "Policies", "Create"}
	case editPolicy:
		segments = []string{"Home", "Access", "Policies", "Edit"}
	case deletePolicy:
		segments = []string{"Home", "Access", "Policies", "Delete"}
	case selectPolicyClient:
		segments = []string{"Home", "Access", "Policies", "Select Client"}
	case policyExport:
		segments = []string{"Home", "Access", "Policies", "Export"}
	case unlockScreen:
		segments = []string{"Home", "Unlock"}
	default:
		return ""
	}

	sep := breadcrumbSepStyle.Render(" › ")
	var parts []string
	for i, s := range segments {
		if i == len(segments)-1 {
			parts = append(parts, breadcrumbActiveStyle.Render(s))
		} else {
			parts = append(parts, breadcrumbSegmentStyle.Render(s))
		}
	}

	crumb := strings.Join(parts, sep)
	return breadcrumbBarStyle.Width(m.width).Render(crumb)
}

func (m *model) renderPolicyListView() string {
	if len(m.policies) == 0 {
		emptyMsg := policyErrorStyle.Render("No policies found")
		helpMsg := policyDescStyle.Render("Press 'b' to go back")

		return lipgloss.JoinVertical(
			lipgloss.Center,
			emptyMsg,
			helpMsg,
		)
	}

	l := renderPolicyList(m.policies, m.width, m.height)
	countMsg := policySuccessStyle.Render(fmt.Sprintf("%d policies loaded", len(m.policies)))

	return lipgloss.JoinVertical(lipgloss.Center, l.View(), countMsg)
}

func (m *model) renderClientSelectorView() string {
	if len(m.clients) == 0 {
		emptyMsg := policyErrorStyle.Render("No clients found")
		helpMsg := policyDescStyle.Render("Press 'b' to go back")

		return lipgloss.JoinVertical(
			lipgloss.Center,
			emptyMsg,
			helpMsg,
		)
	}

	// Use createClientSelector to build a huh-form based selector
	selectorTitle := "Select Client"
	switch m.policyOperation {
	case "view":
		selectorTitle = "Select client to view policy"
	case "edit":
		selectorTitle = "Select client to edit policy"
	case "create":
		selectorTitle = "Select client to create policy"
	case "delete":
		selectorTitle = "Select client to delete policy"
	case "export":
		selectorTitle = "Select client to export policy"
	}

	selectorForm := createClientSelector(m.clients, selectorTitle)
	return lipgloss.JoinVertical(lipgloss.Center, selectorForm.View())
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
