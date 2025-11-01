package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// UnlockMsg is a message that signals the main TUI that unlock should be attempted.
type UnlockMsg struct {
	Passphrase string
}

// unlockFormModel represents the state of the form for unlocking Gaia.
type unlockFormModel struct {
	form   *huh.Form
	width  int
	height int
}

func newUnlockFormModel() *unlockFormModel {
	var passphrase string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("passphrase").
				Title(lipgloss.NewStyle().Bold(true).Render("Enter Passphrase")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("Enter your master passphrase").
				EchoMode(huh.EchoModePassword).
				Value(&passphrase),
		),
	).WithWidth(50)

	return &unlockFormModel{
		form: form,
	}
}

func (m *unlockFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *unlockFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check for an escape key to go back (for forms, we only use esc, not 'b')
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		return m, func() tea.Msg {
			return BackMsg{}
		}
	}

	var updatedForm tea.Model
	var cmd tea.Cmd

	updatedForm, cmd = m.form.Update(msg)
	m.form = updatedForm.(*huh.Form)

	if m.form.State == huh.StateCompleted {
		passphrase := m.form.GetString("passphrase")
		return m, func() tea.Msg {
			return UnlockMsg{
				Passphrase: passphrase,
			}
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m *unlockFormModel) View() string {
	return m.form.View()
}
