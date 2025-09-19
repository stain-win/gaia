package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// AddRecordMsg is a message that signals the main TUI that a record has been added.
type AddRecordMsg struct {
	ClientName string
	Namespace  string
	Key        string
	Value      string
}

// addRecordFormModel represents the state of the form for adding a new secret.
type addRecordFormModel struct {
	form   *huh.Form
	width  int
	height int
}

func newAddRecordFormModel(clients []string, namespaces []string) *addRecordFormModel {
	var clientName, namespace, key, value string

	clientOptions := make([]huh.Option[string], len(clients))
	for i, c := range clients {
		clientOptions[i] = huh.NewOption(c, c)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("clientName").
				Title(lipgloss.NewStyle().Bold(true).Render("Client")).
				Options(clientOptions...).
				Value(&clientName),
			huh.NewInput().
				Key("namespace").
				Title(lipgloss.NewStyle().Bold(true).Render("Namespace")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("e.g., 'production' or 'staging'").
				Value(&namespace),
			huh.NewInput().
				Key("key").
				Title(lipgloss.NewStyle().Bold(true).Render("Key")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("e.g., 'database_password'").
				Value(&key),
			huh.NewInput().
				Key("value").
				Title(lipgloss.NewStyle().Bold(true).Render("Value")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("e.g., 'super-secret-string'").
				Value(&value),
		),
	).WithWidth(40)

	return &addRecordFormModel{
		form: form,
	}
}

func (m *addRecordFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *addRecordFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var updatedForm tea.Model

	updatedForm, cmd = m.form.Update(msg)
	if f, ok := updatedForm.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		// When the form is submitted, send a message to the main TUI.
		return m, func() tea.Msg {
			return AddRecordMsg{
				ClientName: strings.TrimSpace(m.form.GetString("clientName")),
				Namespace:  strings.TrimSpace(m.form.GetString("namespace")),
				Key:        strings.TrimSpace(m.form.GetString("key")),
				Value:      strings.TrimSpace(m.form.GetString("value")),
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

func (m *addRecordFormModel) View() string {
	return m.form.View()
}
