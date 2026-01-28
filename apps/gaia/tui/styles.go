package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - consistent theme colors
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#6366F1") // Indigo
	colorAccent    = lipgloss.Color("#FF8C00") // Orange

	colorSuccess = lipgloss.Color("#10B981") // Green
	colorWarning = lipgloss.Color("#F59E0B") // Amber
	colorError   = lipgloss.Color("#EF4444") // Red

	colorText      = lipgloss.Color("#E5E7EB")
	colorTextMuted = lipgloss.Color("#9CA3AF")
	colorBorder    = lipgloss.Color("#4B5563")
	colorBgDark    = lipgloss.Color("#1F2937")
	colorBgDarker  = lipgloss.Color("#111827")
)

// titleStyle is used for the main menu title
var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorAccent).
	PaddingLeft(1)

// Status bar styles
var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgDark).
			Padding(0, 1)

	// Status badges with high visibility
	statusBadgeUnlocked = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(colorSuccess).
				Bold(true).
				Padding(0, 1)

	statusBadgeLocked = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(colorWarning).
				Bold(true).
				Padding(0, 1)

	statusBadgeOffline = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorError).
				Bold(true).
				Padding(0, 1)

	// Message styles
	messageStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Italic(true)

	successMessageStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	warningMessageStyle = lipgloss.NewStyle().
				Foreground(colorWarning)
)

// Help bar styles
var (
	helpBarStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Background(colorBgDarker).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted)
)

// Content box styles
var (
	contentBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)
)
