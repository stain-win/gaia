package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
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
			Padding(1)

	focusedPaneStyle = paneStyle.
				BorderForeground(lipgloss.Color("69"))
)

type inspectorPane int

const (
	clientsPane inspectorPane = iota
	secretsPane
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

	// Data
	allData        map[string][]*pb.Namespace // clientName -> namespaces
	selectedClient string
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
			items = append(items, namespaceItem(client))
		}
		m.clientsList.SetItems(items)
		// Fetch secrets for the first client automatically
		if len(msg.clients) > 0 {
			m.selectedClient = msg.clients[0]
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
		// Switch focus between panes
		if msg.String() == "tab" {
			m.focusedPane = (m.focusedPane + 1) % 2
			return m, nil
		}

		// Handle selection in the clients list
		if m.focusedPane == clientsPane {
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
		}
	}

	// Delegate updates to the focused component
	if m.focusedPane == clientsPane {
		m.clientsList, cmd = m.clientsList.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.secretsList, cmd = m.secretsList.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update the viewport content when the secrets list selection changes
	if m.secretsList.SelectedItem() != nil {
		if secret, ok := m.secretsList.SelectedItem().(secretItem); ok {

			key := secret.key
			value := secret.value
			secret := fmt.Sprintf("Key: %s\n\nValue:\n%s", key, value)
			m.viewport.SetContent(secret)
		}
	} else {
		m.viewport.SetContent("")
	}

	return m, tea.Batch(cmds...)
}

// View renders the three-pane layout.
func (m *inspectorModel) View() string {
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

// updateSecretsList populates the secrets list based on the selected client.
func (m *inspectorModel) updateSecretsList() {
	var items []list.Item
	namespaces := m.allData[m.selectedClient]
	for _, ns := range namespaces {
		for _, secret := range ns.Secrets {
			items = append(items, secretItem{
				namespace: ns.Name,
				key:       secret.Id,
				value:     secret.Value,
			})
		}
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

// secretItem represents an item in the secrets list.
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
type namespaceItem string

func (n namespaceItem) Title() string       { return string(n) }
func (n namespaceItem) Description() string { return "Client" }
func (n namespaceItem) FilterValue() string { return string(n) }
