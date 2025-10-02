package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

var (
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	focusedPaneStyle = paneStyle.BorderForeground(lipgloss.Color("69"))
)

type inspectorPane int

const (
	clientsPane inspectorPane = iota
	secretsPane
	viewPane
)

// inspectorModel holds the state for our three-pane view.
type inspectorModel struct {
	config *config.Config
	width  int
	height int

	focusedPane inspectorPane
	clientsList list.Model
	secretsList list.Model
	viewport    viewport.Model
	tbl         table.Model

	allData           map[string][]*pb.Namespace
	selectedClient    string
	lastNamespaceName string // To restore selection after updates
	statusMessage     string

	// Edit form state
	editing       bool
	editForm      *huh.Form
	editKey       string
	editValue     string
	editNamespace string
}

func newInspectorModel(cfg *config.Config) *inspectorModel {
	clientsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	clientsList.Title = "Clients"
	clientsList.SetShowHelp(false)

	secretsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	secretsList.Title = "Namespaces"
	secretsList.SetShowHelp(false)

	vp := viewport.New(0, 0)

	return &inspectorModel{
		config:            cfg,
		clientsList:       clientsList,
		secretsList:       secretsList,
		viewport:          vp,
		focusedPane:       clientsPane,
		allData:           make(map[string][]*pb.Namespace),
		lastNamespaceName: "",
	}
}

func (m *inspectorModel) Init() tea.Cmd {
	return fetchAllClientsCmd(m.config)
}

// Update is the main message handler for the inspector view.
func (m *inspectorModel) Update(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	if m.editing {
		return m.updateEditView(msg)
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case allClientsLoadedMsg:
		return m.handleClientsLoaded(msg)

	case secretsForClientLoadedMsg:
		return m.handleSecretsLoaded(msg)

	case recordAddedMsg: // Handle the result of the update
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v. Reverting.", msg.err)
			// Re-fetch to get the true state from the server
			return m, fetchSecretsForClientCmd(m.config, m.selectedClient)
		}
		m.statusMessage = "Secret updated successfully!"
		// No need to re-fetch, optimistic update was successful
		m.lastNamespaceName = "" // Clear the saved name
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Back):
			return m, func() tea.Msg { return backToDataManagementMsg{} }
		case key.Matches(msg, keys.Tab):
			return m.cycleFocus(true)
		case key.Matches(msg, keys.ShiftTab):
			return m.cycleFocus(false)
		}
	}

	var cmd tea.Cmd
	switch m.focusedPane {
	case clientsPane:
		cmd = m.updateClientsPane(msg)
	case secretsPane:
		cmd = m.updateSecretsPane(msg)
	case viewPane:
		cmd = m.updateViewPane(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// updateEditView handles all updates when the edit form is active.
func (m *inspectorModel) updateEditView(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	var cmds []tea.Cmd
	form, cmd := m.editForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.editForm = f
	}
	cmds = append(cmds, cmd)

	if m.editForm.State == huh.StateCompleted {
		newValue := m.editForm.GetString("value")
		m.editing = false
		m.editValue = newValue // Store for optimistic update

		// Optimistically update local data
		if namespaces, ok := m.allData[m.selectedClient]; ok {
			for _, ns := range namespaces {
				if ns.Name == m.editNamespace {
					for _, secret := range ns.Secrets {
						if secret.Id == m.editKey {
							secret.Value = m.editValue
							break
						}
					}
					break
				}
			}
		}
		// Refresh the view from local data
		m.updateSecretsList()

		// Send the update to the daemon
		cmds = append(cmds, addRecordToDaemonCmd(m.config, m.selectedClient, m.editNamespace, m.editKey, newValue))
	} else if m.editForm.State == huh.StateAborted {
		m.editing = false
		m.statusMessage = "Edit cancelled."
		m.lastNamespaceName = ""
	}

	return m, tea.Batch(cmds...)
}

// updateClientsPane handles updates when the clients list is focused.
func (m *inspectorModel) updateClientsPane(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.clientsList, cmd = m.clientsList.Update(msg)

	if m.clientsList.SelectedItem() != nil {
		newClient := m.clientsList.SelectedItem().(listItem).FilterValue()
		if newClient != m.selectedClient {
			m.selectedClient = newClient
			m.secretsList.SetItems([]list.Item{}) // Clear previous items
			m.viewport.SetContent("")
			m.lastNamespaceName = "" // Reset when client changes

			if _, ok := m.allData[m.selectedClient]; !ok {
				return fetchSecretsForClientCmd(m.config, m.selectedClient)
			}
			m.updateSecretsList()
		}
	}
	return cmd
}

// updateSecretsPane handles updates when the secrets list is focused.
func (m *inspectorModel) updateSecretsPane(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, keys.Enter) {
			_, cmd = m.cycleFocus(true) // Cycle forward to the view pane
			return cmd
		}
	}

	m.secretsList, cmd = m.secretsList.Update(msg)
	m.updateTableView()
	return cmd
}

// updateViewPane handles updates when the value viewport is focused.
func (m *inspectorModel) updateViewPane(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	m.viewport.SetContent(m.tbl.View())

	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, keys.Enter) {
		row := m.tbl.SelectedRow()
		if len(row) == 2 {
			m.editing = true
			if nsItem, ok := m.secretsList.SelectedItem().(namespaceListItem); ok {
				m.lastNamespaceName = nsItem.name // Save name before editing
				m.editNamespace = nsItem.name
			}
			m.editKey = row[0]
			m.editValue = row[1]

			// Create and initialize the form
			input := huh.NewInput().
				Title(fmt.Sprintf("New value for %s", m.editKey)).
				Value(&m.editValue).Key("value")

			m.editForm = huh.NewForm(huh.NewGroup(input)).WithTheme(huh.ThemeBase())
			return m.editForm.Init()
		}
	}
	return cmd
}

// cycleFocus moves the focus between the three panes.
func (m *inspectorModel) cycleFocus(forward bool) (*inspectorModel, tea.Cmd) {
	if forward {
		m.focusedPane = (m.focusedPane + 1) % 3
	} else {
		// Go backwards, wrapping around
		m.focusedPane = (m.focusedPane - 1 + 3) % 3
	}

	if m.focusedPane == viewPane {
		m.tbl.Focus()
	} else {
		m.tbl.Blur()
	}
	return m, nil
}

// handleClientsLoaded processes the message with the list of all clients.
func (m *inspectorModel) handleClientsLoaded(msg allClientsLoadedMsg) (*inspectorModel, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	var items []list.Item
	for _, client := range msg.clients {
		items = append(items, listItem{title: client.Name, description: "Client"})
	}
	m.clientsList.SetItems(items)

	if len(msg.clients) > 0 {
		m.selectedClient = msg.clients[0].Name
		return m, fetchSecretsForClientCmd(m.config, m.selectedClient)
	}
	return m, nil
}

// handleSecretsLoaded processes the message with secrets for a specific client.
func (m *inspectorModel) handleSecretsLoaded(msg secretsForClientLoadedMsg) (*inspectorModel, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.allData[msg.clientName] = msg.namespaces
	if msg.clientName == m.selectedClient {
		m.updateSecretsList()
	}
	return m, nil
}

// updateSecretsList populates the secrets list based on the selected client.
func (m *inspectorModel) updateSecretsList() {
	var items []list.Item
	namespaces := m.allData[m.selectedClient]
	for _, ns := range namespaces {
		items = append(items, namespaceListItem{name: ns.Name, secrets: ns.Secrets})
	}
	m.secretsList.SetItems(items)

	// Restore selection after data refresh
	restoredIndex := 0
	if m.lastNamespaceName != "" {
		for i, item := range items {
			if item.(namespaceListItem).name == m.lastNamespaceName {
				restoredIndex = i
				break
			}
		}
	}

	if len(items) > 0 {
		m.secretsList.Select(restoredIndex)
	}

	m.updateTableView()
}

// updateTableView creates/updates the table based on the selected namespace.
func (m *inspectorModel) updateTableView() {
	if m.secretsList.SelectedItem() == nil {
		m.viewport.SetContent("")
		return
	}

	nsItem, ok := m.secretsList.SelectedItem().(namespaceListItem)
	if !ok {
		return
	}

	var rows [][]string
	for _, secret := range nsItem.secrets {
		rows = append(rows, []string{secret.Id, secret.Value})
	}

	m.tbl = newKeyValueTable(rows, m.viewport.Width, m.viewport.Height)
	m.viewport.SetContent(m.tbl.View())
}

// View renders the three-pane layout.
func (m *inspectorModel) View() string {
	if m.editing {
		return m.renderEditView()
	}

	// Build the main three-pane view
	clientsView := m.clientsList.View()
	secretsView := m.secretsList.View()
	viewportView := m.viewport.View()

	clientsStyle, secretsStyle, viewportStyle := paneStyle, paneStyle, paneStyle
	switch m.focusedPane {
	case clientsPane:
		clientsStyle = focusedPaneStyle
	case secretsPane:
		secretsStyle = focusedPaneStyle
	case viewPane:
		viewportStyle = focusedPaneStyle
	}

	leftPane := lipgloss.JoinVertical(lipgloss.Left,
		clientsStyle.Render(clientsView),
		secretsStyle.Render(secretsView),
	)

	rightPane := viewportStyle.Render(viewportView)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Build the status bar
	statusView := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(lipgloss.Width(mainView)).
		Render(m.statusMessage)

	// Combine main view and status bar
	fullView := lipgloss.JoinVertical(lipgloss.Left, mainView, statusView)

	// Center the entire UI
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		fullView,
	)
}

// renderEditView renders the form for editing a secret's value.
func (m *inspectorModel) renderEditView() string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		paneStyle.Render(m.editForm.View()),
	)
}

func (m *inspectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	contentWidth := w
	if w > 120 {
		contentWidth = 120
	}

	statusBarHeight := 1
	mainHeight := h - statusBarHeight

	leftPaneWidth := contentWidth / 3
	rightPaneWidth := contentWidth - leftPaneWidth

	// paneStyle has 1px border and 1px padding on each side (left/right). Total horizontal overhead is 4.
	listContentWidth := leftPaneWidth - 4

	// The two left panes are stacked. Each has a border.
	// Total vertical border space for the stack is 4.
	availableHeight := mainHeight - 4
	clientsListHeight := availableHeight / 2
	secretsListHeight := availableHeight - clientsListHeight

	m.clientsList.SetSize(listContentWidth, clientsListHeight)
	m.secretsList.SetSize(listContentWidth, secretsListHeight)

	// The right pane takes the full height of the main content area.
	m.viewport.Width = rightPaneWidth - 4
	m.viewport.Height = mainHeight - 2
}

// newKeyValueTable creates a bubbles/table for key-value pairs.
func newKeyValueTable(rows [][]string, width, height int) table.Model {
	// table includes 1 separator and 2 padding per cell (4 total)
	// total overhead is 5
	availableWidth := width - 5
	keyWidth := availableWidth / 4
	valueWidth := availableWidth - keyWidth

	columns := []table.Column{
		{Title: "KEY", Width: keyWidth},
		{Title: "VALUE", Width: valueWidth},
	}

	tableRows := make([]table.Row, len(rows))
	for i, r := range rows {
		if len(r) == 2 {
			tableRows[i] = table.Row{r[0], r[1]}
		}
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	tbl.SetStyles(s)

	return tbl
}

// listItem is a generic item for bubbletea lists.
type listItem struct {
	title, description string
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.description }
func (i listItem) FilterValue() string { return i.title }

// namespaceListItem represents an item in the secrets list.
type namespaceListItem struct {
	name    string
	secrets []*pb.Secret
}

func (i namespaceListItem) Title() string       { return i.name }
func (i namespaceListItem) Description() string { return fmt.Sprintf("%d secrets", len(i.secrets)) }
func (i namespaceListItem) FilterValue() string { return i.name }
