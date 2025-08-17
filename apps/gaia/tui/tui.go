package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stain-win/gaia/apps/gaia/config"
)

// Global ASCII art for the Gaia logo.
const gaiaLogo = `

      ___           ___                       ___     
     /\  \         /\  \          ___        /\  \    
    /::\  \       /::\  \        /\  \      /::\  \   
   /:/\:\  \     /:/\:\  \       \:\  \    /:/\:\  \  
  /:/  \:\  \   /::\~\:\  \      /::\__\  /::\~\:\  \ 
 /:/__/_\:\__\ /:/\:\ \:\__\  __/:/\/__/ /:/\:\ \:\__\
 \:\  /\ \/__/ \/__\:\/:/  / /\/:/  /    \/__\:\/:/  /
  \:\ \:\__\        \::/  /  \::/__/          \::/  / 
   \:\/:/  /        /:/  /    \:\__\          /:/  /  
    \::/  /        /:/  /      \/__/         /:/  /   
     \/__/         \/__/                     \/__/    

`

// Run initializes and runs the TUI application.
func Run(cfg *config.Config) error {
	p := tea.NewProgram(initialModel(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
