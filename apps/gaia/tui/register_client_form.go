package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// RegisterClientMsg is a message that signals the main TUI that a new client needs to be registered.
type RegisterClientMsg struct {
	ClientName string
}

// registerClientFormModel represents the state of the form for registering a new client.
type registerClientFormModel struct {
	form   *huh.Form
	width  int
	height int
}

func newRegisterClientFormModel() *registerClientFormModel {
	var clientName string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("clientName").
				Title(lipgloss.NewStyle().Bold(true).Render("Client Common Name")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("e.g., 'web-app-A'").
				Value(&clientName),
		),
	).WithWidth(40)

	return &registerClientFormModel{
		form: form,
	}
}

func (m *registerClientFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *registerClientFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var updatedForm tea.Model

	updatedForm, cmd = m.form.Update(msg)
	m.form = updatedForm.(*huh.Form)

	if m.form.State == huh.StateCompleted {
		return m, func() tea.Msg {
			return RegisterClientMsg{
				ClientName: strings.TrimSpace(m.form.GetString("clientName")),
			}
		}
	}

	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "b" {
		return m, func() tea.Msg {
			return BackMsg{}
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m *registerClientFormModel) View() string {
	return m.form.View()
}
