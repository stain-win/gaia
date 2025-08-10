package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// AddRecordMsg is a message that signals the main TUI that a record has been added.
type AddRecordMsg struct {
	Namespace string
	Key       string
	Value     string
}

// addRecordFormModel represents the state of the form for adding a new secret.
type addRecordFormModel struct {
	form      *huh.Form
	namespace string
	key       string
	value     string
	width     int
	height    int
}

func newAddRecordFormModel() *addRecordFormModel {
	var namespace, key, value string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("namespace").
				Title(lipgloss.NewStyle().Bold(true).Render("Namespace")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render(">")).
				Placeholder("e.g., 'production/api'").
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
		form:      form,
		namespace: namespace,
		key:       key,
		value:     value,
	}
}

func (m *addRecordFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *addRecordFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var updatedForm tea.Model

	updatedForm, cmd = m.form.Update(msg)
	m.form = updatedForm.(*huh.Form)

	if m.form.State == huh.StateCompleted {
		// When the form is submitted, send a message to the main TUI.
		return m, func() tea.Msg {
			return AddRecordMsg{
				Namespace: strings.TrimSpace(m.form.GetString("namespace")),
				Key:       strings.TrimSpace(m.form.GetString("key")),
				Value:     strings.TrimSpace(m.form.GetString("value")),
			}
		}
	}

	// A special key to go back
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

func (m *addRecordFormModel) View() string {
	return appStyle.Render(
		lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.form.View(),
		),
	)
}
