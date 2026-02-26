// Package tui provides the interactive terminal user interface for Gaia.
// It uses Bubble Tea to create a full-screen TUI for managing secrets, clients,
// and interacting with the Gaia daemon.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stain-win/gaia/apps/gaia/config"
)

// Run initializes and runs the TUI application.
func Run(cfg *config.Config) error {
	p := tea.NewProgram(initialModel(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
