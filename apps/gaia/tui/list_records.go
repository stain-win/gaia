package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

// Define styles for the inspector panes
var (
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).MarginBottom(1)

	focusedPaneStyle = paneStyle.
				BorderForeground(lipgloss.Color("69"))
)

type inspectorPane int
type namespaceItem string

const (
	clientsPane inspectorPane = iota
	secretsPane
	viewPane
)

// inspectorModel holds the state for our new three-pane view.
type inspectorModel struct {
	config *config.Config
	width  int
	height int

	focusedPane inspectorPane
	clientsList list.Model
	secretsList list.Model
	viewport    viewport.Model
	tbl         table.Model // value type, not pointer

	// Data
	allData        map[string][]*pb.Namespace // clientName -> namespaces
	selectedClient string

	// Edit form state
	editing       bool
	editKey       string
	editValue     string
	editNamespace string
}

func newInspectorModel(cfg *config.Config) *inspectorModel {
	clientsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	clientsList.Title = titleStyle.Render("Clients")
	clientsList.SetShowHelp(false)
	clientsList.SetShowFilter(false)

	secretsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	secretsList.Title = titleStyle.Render("Secrets")
	secretsList.SetShowHelp(false)
	secretsList.SetShowFilter(false)

	vp := viewport.New(0, 0)
	vp.Style = paneStyle

	return &inspectorModel{
		config:      cfg,
		clientsList: clientsList,
		secretsList: secretsList,
		viewport:    vp,
		focusedPane: clientsPane,
		allData:     make(map[string][]*pb.Namespace),
	}
}

// Init fetches the initial data.
func (m *inspectorModel) Init() tea.Cmd {
	return fetchAllClientsCmd(m.config)
}

// Update handles messages for the inspector.
func (m *inspectorModel) Update(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recalculate pane sizes
		h, v := listViewStyle.GetFrameSize()
		paneWidth := (msg.Width - h) / 3
		paneHeight := v - 3 // Adjust for title/footer

		m.clientsList.SetSize(paneWidth, paneHeight)
		m.secretsList.SetSize(paneWidth, paneHeight)

		m.viewport.Width = paneWidth
		m.viewport.Height = paneHeight

	case allClientsLoadedMsg:
		if msg.err != nil {
			// Handle error
			return m, nil
		}
		var items []list.Item
		for _, client := range msg.clients {
			items = append(items, namespaceItem(client.Name))
		}
		m.clientsList.SetItems(items)
		// Fetch secrets for the first client automatically
		if len(msg.clients) > 0 {
			m.selectedClient = msg.clients[0].Name
			return m, fetchSecretsForClientCmd(m.config, m.selectedClient)
		}

	case secretsForClientLoadedMsg:
		if msg.err != nil {
			// Handle error
			return m, nil
		}
		m.allData[msg.clientName] = msg.namespaces
		m.updateSecretsList()

	case tea.KeyMsg:
		if msg.String() == "tab" {
			// Cycle focus: clientsPane > secretsPane > viewPane > clientsPane
			if m.focusedPane == clientsPane {
				m.focusedPane = secretsPane
			} else if m.focusedPane == secretsPane {
				m.focusedPane = viewPane
				if m.tbl.Rows() != nil && len(m.tbl.Rows()) > 0 {
					m.tbl.Focus()
					m.viewport.SetContent(m.tbl.View())
				}
			} else if m.focusedPane == viewPane {
				m.focusedPane = clientsPane
			}
			return m, nil // <--- IMMEDIATELY RETURN after tab key
		}
		if key.Matches(msg, keys.Back) {
			return m, func() tea.Msg { return backToDataManagementMsg{} }
		}
		if m.focusedPane == clientsPane {
			// Handle selection in the client's list
			if m.clientsList.SelectedItem() != nil {
				newClient := string(m.clientsList.SelectedItem().(namespaceItem))
				if newClient != m.selectedClient {
					m.selectedClient = newClient
					// Check if we already have data, otherwise fetch it
					if _, ok := m.allData[m.selectedClient]; !ok {
						return m, fetchSecretsForClientCmd(m.config, m.selectedClient)
					}
					m.updateSecretsList()
				}
			}
		} else if m.focusedPane == secretsPane {
			if m.secretsList.SelectedItem() != nil {
				if nsItem, ok := m.secretsList.SelectedItem().(namespaceListItem); ok {
					rows := [][]string{}
					for _, secret := range nsItem.secrets {
						rows = append(rows, []string{secret.Id, secret.Value})
					}
					m.tbl = newKeyValueTable(rows)
					m.viewport.SetContent(m.tbl.View())
				}
			}
		} else if m.focusedPane == viewPane {
			m.tbl, cmd = m.tbl.Update(msg)
			cmds = append(cmds, cmd)
			m.tbl.Focus()
			m.viewport.SetContent(m.tbl.View())
			if msg.Type == tea.KeyEnter {
				row := m.tbl.SelectedRow()
				if len(row) == 2 {
					m.editing = true
					m.editKey = row[0]
					m.editValue = row[1]
					if nsItem, ok := m.secretsList.SelectedItem().(namespaceListItem); ok {
						m.editNamespace = nsItem.name
					}
					return m, nil
				}
			}
		}
	}

	// Delegate updates to the focused component
	switch m.focusedPane {
	case clientsPane:
		m.clientsList, cmd = m.clientsList.Update(msg)
		cmds = append(cmds, cmd)
	case secretsPane:
		m.secretsList, cmd = m.secretsList.Update(msg)
		cmds = append(cmds, cmd)
	case viewPane:
		m.tbl, cmd = m.tbl.Update(msg)
		cmds = append(cmds, cmd)
		m.tbl.Focus()
		m.viewport.SetContent(m.tbl.View())
	}

	// Update the viewport content when the secret list selection changes
	if m.secretsList.SelectedItem() != nil {
		if nsItem, ok := m.secretsList.SelectedItem().(namespaceListItem); ok {
			rows := [][]string{}
			for _, secret := range nsItem.secrets {
				rows = append(rows, []string{secret.Id, secret.Value})
			}
			m.tbl = newKeyValueTable(rows)
			if m.focusedPane == viewPane {
				m.tbl.Focus()
			}
			m.viewport.SetContent(m.tbl.View())
		}
	} else {
		m.viewport.SetContent("")
	}

	return m, tea.Batch(cmds...)
}

// Helper to create a bubbles/table for key-value pairs
func newKeyValueTable(rows [][]string) table.Model {
	columns := []table.Column{
		{Title: "KEY", Width: 24},
		{Title: "VALUE", Width: 40},
	}
	tableRows := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		if len(r) == 2 {
			tableRows = append(tableRows, table.Row{r[0], r[1]})
		}
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	tblStyle := table.DefaultStyles()

	tblStyle.Header = tblStyle.Header.BorderStyle(lipgloss.ASCIIBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	tblStyle.Cell = tblStyle.Cell.Margin(1, 0)

	tblStyle.Selected = tblStyle.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false).Height(1)

	tbl.SetStyles(tblStyle)
	return tbl
}

// View renders the three-pane layout.
func (m *inspectorModel) View() string {
	if m.editing {
		return renderEditForm(m.editKey, m.editValue)
	}
	var clientsPaneView, secretsPaneView, sidePaneView, valuePaneView string

	clientsPaneStyle := paneStyle.Width(m.clientsList.Width()).Height(m.clientsList.Height())
	secretsPaneStyle := paneStyle.Width(m.secretsList.Width()).Height(m.secretsList.Height())

	if m.focusedPane == clientsPane {
		clientsPaneStyle = focusedPaneStyle.Width(m.clientsList.Width()).Height(m.clientsList.Height())
	} else {
		secretsPaneStyle = focusedPaneStyle.Width(m.secretsList.Width()).Height(m.secretsList.Height())
	}

	clientsPaneView = clientsPaneStyle.Render(m.clientsList.View())
	secretsPaneView = secretsPaneStyle.Render(m.secretsList.View())
	sidePaneView = lipgloss.JoinVertical(lipgloss.Left, clientsPaneView, secretsPaneView)
	valuePaneView = m.viewport.View()

	return lipgloss.NewStyle().Height(m.height).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, sidePaneView, valuePaneView),
	)
}

// updateSecretsList populates the secret list based on the selected client.
func (m *inspectorModel) updateSecretsList() {
	var items []list.Item
	namespaces := m.allData[m.selectedClient]
	for _, ns := range namespaces {
		items = append(items, namespaceListItem{
			name:    ns.Name,
			secrets: ns.Secrets,
		})
	}
	m.secretsList.SetItems(items)
}

func (m *inspectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	paneWidth := (w / 3) - 2

	const paneVerticalPadding = 2 // 1 for top padding, 1 for bottom
	listHeight := h/2 - paneVerticalPadding

	m.clientsList.SetSize(paneWidth, min(listHeight, 10))
	m.secretsList.SetSize(paneWidth, h-(m.clientsList.Height()+paneVerticalPadding+1))
	m.viewport.Width = paneWidth * 2
	m.viewport.Height = listHeight
}

// secretItem represents an item in the secret list.
type secretItem struct {
	namespace, key, value string
}

func (i secretItem) Title() string {
	return fmt.Sprintf("%s", i.namespace)
}
func (i secretItem) Description() string {
	return "namespace"
}
func (i secretItem) FilterValue() string { return i.key }

// namespaceItem is a custom list.Item for the clients' pane.

func (n namespaceItem) Title() string       { return string(n) }
func (n namespaceItem) Description() string { return "Client" }
func (n namespaceItem) FilterValue() string { return string(n) }

// namespaceListItem represents an item in the secrets list for a namespace.
type namespaceListItem struct {
	name    string
	secrets []*pb.Secret
}

func (i namespaceListItem) Title() string       { return i.name }
func (i namespaceListItem) Description() string { return "Namespace" }
func (i namespaceListItem) FilterValue() string { return i.name }

// Dummy edit form renderer (replace with real implementation)
func renderEditForm(key, value string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00BFFF")).Render(
		fmt.Sprintf("Edit value for key: %s\nCurrent value: %s\n[Implement input here]", key, value),
	)
}
