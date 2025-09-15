package tui

import "github.com/charmbracelet/lipgloss"

// Global styles for the TUI.
var (
	appStyle = lipgloss.NewStyle().
			Width(80).
			Align(lipgloss.Center).AlignVertical(lipgloss.Top)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF8C00")). // Orange
			PaddingLeft(1)
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")). // Deep Sky Blue
			PaddingLeft(1)
	helpStyle = lipgloss.NewStyle().
			AlignHorizontal(lipgloss.Center).MarginTop(1).
			Foreground(lipgloss.Color("241")). // Light Gray
			MarginTop(1)                       // Add one line of top padding

	inputFieldStyle = lipgloss.NewStyle().
			BorderForeground().
			BorderStyle(lipgloss.NormalBorder()).Padding(1).Width(30)

	listViewStyle = lipgloss.NewStyle().
			PaddingRight(1).
			MarginRight(1).
			Border(lipgloss.RoundedBorder(), false, true, false, false)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
			Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})
)

// Style represents a reusable lipgloss style.
type Style struct {
	BorderColor   *lipgloss.Color
	Foreground    *lipgloss.Color
	Background    *lipgloss.Color
	InputField    *lipgloss.Style
	Padding       *int
	Margin        *int
	Bold          *bool
	Italic        *bool
	Underline     *bool
	Strikethrough *bool
}
