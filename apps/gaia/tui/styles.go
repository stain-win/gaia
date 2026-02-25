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

	// Message banner styles
	messageBannerBase = lipgloss.NewStyle().
				Padding(0, 2).
				Bold(true)

	errorBannerStyle = messageBannerBase.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#7F1D1D")).
				BorderForeground(colorError)

	successBannerStyle = messageBannerBase.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#064E3B")).
				BorderForeground(colorSuccess)

	warningBannerStyle = messageBannerBase.
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#78350F")).
				BorderForeground(colorWarning)

	infoBannerStyle = messageBannerBase.
			Foreground(colorText).
			Background(colorBgDark)

	// Legacy message styles (kept for compatibility)
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
// Title banner style (blue/purple background)
var viewerTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFFFF")).
	Background(lipgloss.Color("#6366F1")).
	Bold(true).
	Padding(0, 1)

// Subtitle style (grey text)
var viewerSubtitleStyle = lipgloss.NewStyle().
	Foreground(colorTextMuted).
	Padding(1, 0, 1, 0)

// Custom list item styles
var (
	itemTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	itemDescStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted)

	selectedItemTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF79C6")).
				Bold(true)

	selectedItemDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D5589E"))

	paneBorderSelectedStyle = lipgloss.NewStyle().
				Padding(0, 1)

	paneBorderUnselectedStyle = lipgloss.NewStyle().
					Padding(0, 1)

	detailKeyStyle = lipgloss.NewStyle().
			Foreground(colorTextMuted)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(colorBorder)

	statusBarTextStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted).
				Padding(0, 1)

	statusBarReadyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#EF4444")).
				Bold(true).
				Padding(0, 1)
)

// Content box styles
var (
	contentBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)
)

// Breadcrumb styles
var (
	breadcrumbBarStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted).
				Background(colorBgDarker).
				Padding(0, 1)

	breadcrumbSegmentStyle = lipgloss.NewStyle().
				Foreground(colorTextMuted)

	breadcrumbActiveStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	breadcrumbSepStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4B5563"))
)
