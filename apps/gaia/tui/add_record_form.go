package tui

import (
	"fmt"
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
	ready     bool
	err       error
	Msg       string
}

func newAddRecordFormModel(namespaces []string) *addRecordFormModel {
	var namespace, key, value string

	options := make([]huh.Option[string], len(namespaces))
	for i, ns := range namespaces {
		options[i] = huh.NewOption(ns, ns)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Key("namespace").
				Title(lipgloss.NewStyle().Bold(true).Render("Namespace")).
				Options(options...).
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
		ready:     true,
		Msg:       "",
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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, cmd
}

func (m *addRecordFormModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	return m.form.View()
}
