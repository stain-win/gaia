package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// nameRule mirrors the server-side validation regex.
var nameRule = regexp.MustCompile(`^[a-z0-9]([-_a-z0-9]{0,61}[a-z0-9])?$`)

func validateName(label, v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("%s is required", label)
	}
	if !nameRule.MatchString(v) {
		return fmt.Errorf("%s: 1-63 chars, lowercase a-z/0-9/hyphens/underscores, must start and end with a letter or number", label)
	}
	return nil
}

func validateNamespace(v string) error {
	v = strings.TrimSpace(v)
	if v == "common" {
		return fmt.Errorf("'common' is a reserved namespace — use the 'common' client instead")
	}
	return validateName("namespace", v)
}

// AddRecordMsg is a message that signals the main TUI that a record has been added.
type AddRecordMsg struct {
	ClientName string
	Namespace  string
	Key        string
	Value      string
}

// addRecordFormModel represents the state of the form for adding a new secret.
type addRecordFormModel struct {
	form    *huh.Form
	clients []string
	width   int
	height  int
	success bool
}

func newAddRecordFormModel(clients []string) *addRecordFormModel {
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
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render("> ")).
				Placeholder("e.g., production or staging").
				Value(&namespace).
				Validate(validateNamespace),
			huh.NewInput().
				Key("key").
				Title(lipgloss.NewStyle().Bold(true).Render("Key")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render("> ")).
				Placeholder("e.g., database-password").
				Value(&key).
				Validate(func(v string) error { return validateName("key", v) }),
			huh.NewInput().
				Key("value").
				Title(lipgloss.NewStyle().Bold(true).Render("Value")).
				Prompt(lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")).Render("> ")).
				Placeholder("the secret value").
				Value(&value).
				Validate(func(v string) error {
					if strings.TrimSpace(v) == "" {
						return fmt.Errorf("value is required")
					}
					return nil
				}),
		),
	).WithWidth(60)

	return &addRecordFormModel{
		form:    form,
		clients: clients,
		success: false,
	}
}

func (m *addRecordFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *addRecordFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.success {
		if keyMsg, ok := msg.(tea.KeyMsg); ok && (keyMsg.Type == tea.KeyEnter || keyMsg.String() == "enter") {
			m.resetForm()
			return m, m.form.Init()
		}
		return m, nil
	}

	var cmd tea.Cmd
	var updatedForm tea.Model

	updatedForm, cmd = m.form.Update(msg)
	if f, ok := updatedForm.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		m.success = true
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
	if m.success {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF00")).Render(
			"Record successfully added!\n\nPress Enter to add another record.")
	}
	return m.form.View()
}

func (m *addRecordFormModel) resetForm() {
	rebuilt := newAddRecordFormModel(m.clients)
	m.form = rebuilt.form
	m.success = false
}
