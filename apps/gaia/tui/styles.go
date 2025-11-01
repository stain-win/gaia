package tui

import "github.com/charmbracelet/lipgloss"

// titleStyle is used for the main menu title
var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FF8C00")). // Orange
	PaddingLeft(1)
