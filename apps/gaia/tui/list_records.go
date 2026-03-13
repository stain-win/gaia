package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/config"
	pb "github.com/stain-win/gaia/apps/gaia/proto"
)

type inspectorPane int

const (
	clientsPane inspectorPane = iota
	secretsPane
)

// secretItem represents a single secret in the global list
type secretItem struct {
	client    string
	namespace string
	id        string
	value     string
}

func (i secretItem) Title() string { return i.id }
func (i secretItem) Description() string {
	return fmt.Sprintf("key: %d bytes, value: %d bytes", len(i.id), len(i.value))
}
func (i secretItem) FilterValue() string { return i.id }

// customDelegate renders list items to match the Redis viewer style
type customDelegate struct {
	focused bool
}

func (d customDelegate) Height() int                             { return 2 }
func (d customDelegate) Spacing() int                            { return 1 }
func (d customDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(secretItem)
	if !ok {
		return
	}

	title := i.Title()
	desc := i.Description()
	if len(desc) > 30 {
		desc = desc[:27] + "..."
	}

	var str string
	if index == m.Index() {
		if d.focused {
			str = lipgloss.JoinVertical(lipgloss.Left,
				selectedItemTitleStyle.Render(title),
				selectedItemDescStyle.Render(desc),
			)
		} else {
			str = lipgloss.JoinVertical(lipgloss.Left,
				itemTitleStyle.Foreground(colorTextMuted).Render(title),
				itemDescStyle.Render(desc),
			)
		}
	} else {
		str = lipgloss.JoinVertical(lipgloss.Left,
			itemTitleStyle.Render(title),
			itemDescStyle.Render(desc),
		)
	}
	_, _ = fmt.Fprintf(w, "%s", str)
}

// clientListItem renders the client selector list
type clientListItem struct {
	name string
}

func (i clientListItem) Title() string       { return i.name }
func (i clientListItem) Description() string { return "" }
func (i clientListItem) FilterValue() string { return i.name }

type inspectorModel struct {
	config *config.Config
	width  int
	height int

	focusedPane inspectorPane
	clientsList list.Model
	secretsList list.Model
	cDelegate   list.DefaultDelegate
	sDelegate   customDelegate
	viewport    viewport.Model

	allClients     []*pb.Client
	pendingClients int
	allData        map[string][]*pb.Namespace
	selectedClient string
	allSecrets     []secretItem
	statusMessage  string

	// Edit form state
	editing       bool
	editForm      *huh.Form
	editKey       string
	editValue     string
	editNamespace string
	editClient    string

	// Delete confirmation state
	deleting        bool
	deleteForm      *huh.Form
	deleteKey       string
	deleteNamespace string
	deleteClient    string
}

func newInspectorModel(cfg *config.Config) *inspectorModel {
	// Simple delegate for the client list
	cDelegate := list.NewDefaultDelegate()
	cDelegate.ShowDescription = false
	cDelegate.SetSpacing(0)
	cDelegate.Styles.SelectedTitle = cDelegate.Styles.SelectedTitle.Foreground(colorTextMuted).BorderForeground(colorTextMuted)

	clientsList := list.New([]list.Item{}, cDelegate, 0, 0)
	clientsList.SetShowTitle(false)
	clientsList.SetShowHelp(false)
	clientsList.SetShowStatusBar(false)

	sDelegate := customDelegate{focused: true} // defaults to secretsPane focus
	secretsList := list.New([]list.Item{}, sDelegate, 0, 0)
	secretsList.SetShowTitle(false)
	secretsList.SetShowHelp(false)
	secretsList.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	return &inspectorModel{
		config:      cfg,
		focusedPane: secretsPane, // Default focus on secrets
		clientsList: clientsList,
		secretsList: secretsList,
		cDelegate:   cDelegate,
		sDelegate:   sDelegate,
		viewport:    vp,
		allData:     make(map[string][]*pb.Namespace),
		allSecrets:  []secretItem{},
	}
}

func (m *inspectorModel) Init() tea.Cmd {
	return fetchAllClientsCmd(m.config)
}

func (m *inspectorModel) updateFocusStates() {
	if m.focusedPane == clientsPane {
		m.cDelegate.Styles.SelectedTitle = m.cDelegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#FF79C6")).BorderForeground(lipgloss.Color("#FF79C6"))
	} else {
		m.cDelegate.Styles.SelectedTitle = m.cDelegate.Styles.SelectedTitle.Foreground(colorTextMuted).BorderForeground(colorTextMuted)
	}
	m.clientsList.SetDelegate(m.cDelegate)

	if m.focusedPane == secretsPane {
		m.sDelegate.focused = true
	} else {
		m.sDelegate.focused = false
	}
	m.secretsList.SetDelegate(m.sDelegate)
}

func (m *inspectorModel) Update(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	if m.deleting {
		return m.updateDeleteView(msg)
	}
	if m.editing {
		return m.updateEditView(msg)
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case allClientsLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error loading clients: %v", msg.err)
			return m, nil
		}

		m.allClients = msg.clients

		items := make([]list.Item, len(m.allClients))
		for i, c := range m.allClients {
			items[i] = clientListItem{name: c.Name}
		}
		m.clientsList.SetItems(items)

		m.pendingClients = len(m.allClients)
		m.allData = make(map[string][]*pb.Namespace)

		if m.pendingClients == 0 {
			m.statusMessage = "Ready | 0 keys found"
			return m, nil
		}

		if len(m.allClients) > 0 {
			m.selectedClient = m.allClients[0].Name
			m.clientsList.Select(0)
		}

		for _, client := range m.allClients {
			cmds = append(cmds, fetchSecretsForClientCmd(m.config, client.Name))
		}
		m.updateFocusStates()

	case secretsForClientLoadedMsg:
		m.pendingClients--
		if msg.err == nil {
			m.allData[msg.clientName] = msg.namespaces
		}

		if m.pendingClients == 0 {
			m.rebuildSecretsList()
		}

	case recordAddedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error: %v. Reverting.", msg.err)
			return m, m.Init()
		}
		m.statusMessage = "Secret updated successfully!"
		return m, nil

	case recordDeletedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error deleting secret: %v. Reverting.", msg.err)
			return m, m.Init()
		}
		m.statusMessage = fmt.Sprintf("Deleted '%s' successfully.", msg.id)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Back):
			if m.focusedPane == secretsPane && m.secretsList.FilterState() != list.Filtering {
				return m, func() tea.Msg { return backToDataManagementMsg{} }
			} else if m.focusedPane == clientsPane && m.clientsList.FilterState() != list.Filtering {
				m.focusedPane = secretsPane
				m.updateFocusStates()
				return m, nil
			}
		case key.Matches(msg, keys.Tab), key.Matches(msg, keys.ShiftTab):
			if m.focusedPane == clientsPane {
				m.focusedPane = secretsPane
			} else {
				m.focusedPane = clientsPane
			}
			m.updateFocusStates()
			return m, nil
		case msg.String() == "d" || msg.String() == "delete":
			if m.focusedPane == secretsPane &&
				m.secretsList.SelectedItem() != nil &&
				m.secretsList.FilterState() != list.Filtering {
				return m.startDeleting()
			}
		case key.Matches(msg, keys.Enter):
			if m.focusedPane == secretsPane && m.secretsList.SelectedItem() != nil {
				return m.startEditing()
			} else if m.focusedPane == clientsPane {
				// Only move to secrets pane if the selected client has secrets
				if len(m.allSecrets) > 0 {
					m.focusedPane = secretsPane
					m.updateFocusStates()
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if m.focusedPane == clientsPane {
		var oldClient string
		if item, ok := m.clientsList.SelectedItem().(clientListItem); ok {
			oldClient = item.name
		}

		m.clientsList, cmd = m.clientsList.Update(msg)

		var newClient string
		if item, ok := m.clientsList.SelectedItem().(clientListItem); ok {
			newClient = item.name
		}

		// If selection changed, immediately update the secrets list
		if newClient != "" && oldClient != newClient {
			m.selectedClient = newClient
			m.rebuildSecretsList()
		}
	} else if m.focusedPane == secretsPane {
		m.secretsList, cmd = m.secretsList.Update(msg)
		m.updateDetailView()
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *inspectorModel) rebuildSecretsList() {
	m.allSecrets = []secretItem{}
	if namespaces, ok := m.allData[m.selectedClient]; ok {
		for _, ns := range namespaces {
			for _, sec := range ns.Secrets {
				m.allSecrets = append(m.allSecrets, secretItem{
					client:    m.selectedClient,
					namespace: ns.Name,
					id:        sec.Id,
					value:     sec.Value,
				})
			}
		}
	}

	items := make([]list.Item, len(m.allSecrets))
	for i, s := range m.allSecrets {
		items[i] = s
	}
	m.secretsList.SetItems(items)
	m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
	m.updateDetailView()
}

func (m *inspectorModel) startEditing() (*inspectorModel, tea.Cmd) {
	item, ok := m.secretsList.SelectedItem().(secretItem)
	if !ok {
		return m, nil
	}

	m.editing = true
	m.editKey = item.id
	m.editValue = item.value
	m.editNamespace = item.namespace
	m.editClient = item.client

	input := huh.NewInput().
		Title(fmt.Sprintf("New value for %s", m.editKey)).
		Value(&m.editValue).Key("value")

	m.editForm = huh.NewForm(huh.NewGroup(input)).WithTheme(huh.ThemeBase())
	return m, m.editForm.Init()
}

func (m *inspectorModel) updateEditView(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	// Intercept ESC before passing to the form for immediate cancel
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.editing = false
		m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
		return m, nil
	}

	var cmds []tea.Cmd
	form, cmd := m.editForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.editForm = f
	}
	cmds = append(cmds, cmd)

	if m.editForm.State == huh.StateCompleted {
		newValue := m.editForm.GetString("value")
		m.editing = false

		// Optimistically update local data
		for i, s := range m.allSecrets {
			if s.id == m.editKey && s.namespace == m.editNamespace && s.client == m.editClient {
				m.allSecrets[i].value = newValue
				break
			}
		}

		// Refresh list items
		items := make([]list.Item, len(m.allSecrets))
		for i, s := range m.allSecrets {
			items[i] = s
		}
		m.secretsList.SetItems(items)
		m.updateDetailView()

		// Send update to daemon
		cmds = append(cmds, addRecordToDaemonCmd(m.config, m.editClient, m.editNamespace, m.editKey, newValue))
	} else if m.editForm.State == huh.StateAborted {
		m.editing = false
		m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
	}

	return m, tea.Batch(cmds...)
}

func (m *inspectorModel) startDeleting() (*inspectorModel, tea.Cmd) {
	item, ok := m.secretsList.SelectedItem().(secretItem)
	if !ok {
		return m, nil
	}

	m.deleting = true
	m.deleteKey = item.id
	m.deleteNamespace = item.namespace
	m.deleteClient = item.client

	var confirmed bool
	m.deleteForm = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirm").
				Title(fmt.Sprintf("Delete secret '%s'?", item.id)).
				Description(fmt.Sprintf("Client: %s  •  Namespace: %s\nThis action is irreversible.", item.client, item.namespace)).
				Affirmative("Yes, delete").
				Negative("No, cancel").
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeBase())
	return m, m.deleteForm.Init()
}

func (m *inspectorModel) updateDeleteView(msg tea.Msg) (*inspectorModel, tea.Cmd) {
	// Intercept ESC for immediate cancel
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.deleting = false
		m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
		return m, nil
	}

	var cmds []tea.Cmd
	form, cmd := m.deleteForm.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.deleteForm = f
	}
	cmds = append(cmds, cmd)

	if m.deleteForm.State == huh.StateCompleted {
		confirmed := m.deleteForm.GetBool("confirm")
		m.deleting = false

		if confirmed {
			// Optimistically remove from local list
			filtered := make([]secretItem, 0, len(m.allSecrets))
			for _, s := range m.allSecrets {
				if !(s.id == m.deleteKey && s.namespace == m.deleteNamespace && s.client == m.deleteClient) {
					filtered = append(filtered, s)
				}
			}
			m.allSecrets = filtered

			items := make([]list.Item, len(m.allSecrets))
			for i, s := range m.allSecrets {
				items[i] = s
			}
			m.secretsList.SetItems(items)
			m.updateDetailView()
			m.statusMessage = fmt.Sprintf("Deleting '%s'...", m.deleteKey)
			cmds = append(cmds, deleteRecordFromDaemonCmd(m.config, m.deleteClient, m.deleteNamespace, m.deleteKey))
		} else {
			m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
		}
	} else if m.deleteForm.State == huh.StateAborted {
		m.deleting = false
		m.statusMessage = fmt.Sprintf("Ready | %d keys found", len(m.allSecrets))
	}

	return m, tea.Batch(cmds...)
}

func (m *inspectorModel) updateDetailView() {
	item, ok := m.secretsList.SelectedItem().(secretItem)
	if !ok {
		m.viewport.SetContent("")
		return
	}

	// Build the detailed view string resembling the Redis viewer
	sb := strings.Builder{}

	sb.WriteString(detailKeyStyle.Render("Key:"))
	sb.WriteString("\n")
	sb.WriteString(detailValueStyle.Render(item.id))
	sb.WriteString("\n")
	sb.WriteString(separatorStyle.Render(strings.Repeat("-", m.viewport.Width)))
	sb.WriteString("\n")

	sb.WriteString(detailKeyStyle.Render("Namespace: "))
	sb.WriteString(detailValueStyle.Render(item.namespace))
	sb.WriteString("\n")
	sb.WriteString(separatorStyle.Render(strings.Repeat("-", m.viewport.Width)))
	sb.WriteString("\n\n")

	sb.WriteString(detailKeyStyle.Render("Value:"))
	sb.WriteString("\n")

	// Try to format value slightly if it looks like JSON or long string, for now just print raw
	sb.WriteString(detailValueStyle.Render(item.value))

	m.viewport.SetContent(sb.String())
}

func (m *inspectorModel) View() string {
	if m.editing {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, paneBorderUnselectedStyle.Render(m.editForm.View()))
	}
	if m.deleting {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, paneBorderUnselectedStyle.Render(m.deleteForm.View()))
	}

	// --- Top Left: Client Selector ---
	clientCount := len(m.clientsList.Items())
	var clientHeader string
	if clientCount > 5 {
		selIdx := m.clientsList.Index() + 1
		clientHeader = viewerSubtitleStyle.Render(fmt.Sprintf(" Clients (%d/%d) ", selIdx, clientCount))
	} else {
		clientHeader = viewerSubtitleStyle.Render(fmt.Sprintf(" Clients (%d) ", clientCount))
	}
	clientsViewBox := lipgloss.JoinVertical(lipgloss.Left, clientHeader, m.clientsList.View())
	// Let's add the horizontal separator using a bottom border on the top-left pane
	topPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorBorder).
		Padding(0, 1). // Small indent to match list
		Width(m.width / 3)

	clientsPaneRender := topPaneStyle.Render(clientsViewBox)

	// --- Bottom Left: Secrets List ---
	titleStr := viewerTitleStyle.Render(" Gaia Secret Viewer ")
	subtitleStr := viewerSubtitleStyle.Render(fmt.Sprintf("%d items", len(m.secretsList.Items())))
	header := lipgloss.JoinVertical(lipgloss.Left, titleStr, subtitleStr)
	listViewBox := lipgloss.JoinVertical(lipgloss.Left, header, m.secretsList.View())

	bottomPaneStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(m.width / 3)

	secretsPaneRender := bottomPaneStyle.Render(listViewBox)

	leftPane := lipgloss.JoinVertical(lipgloss.Left, clientsPaneRender, secretsPaneRender)

	// --- Right Pane: Details ---
	rightPaneWidth := m.width - lipgloss.Width(leftPane)
	borderThickness := 1 // Only left border

	// Draw the vertical separator natively through Lipgloss border
	rightPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorBorder).
		Padding(0, 2).
		Width(rightPaneWidth - borderThickness)

	rightPane := rightPaneStyle.Render(m.viewport.View())

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// --- Bottom Pane (Status Bar) ---
	readyBadge := statusBarReadyStyle.Render("Ready")
	var msg string
	if m.pendingClients > 0 {
		msg = fmt.Sprintf(" Loading... (%d clients remaining) ", m.pendingClients)
	} else {
		msg = fmt.Sprintf(" %d keys found ", len(m.allSecrets))
	}
	statusText := statusBarTextStyle.Render(msg)

	statusBar := lipgloss.JoinHorizontal(lipgloss.Center, readyBadge, statusText)
	barWidth := m.width - lipgloss.Width(statusBar)
	if barWidth > 0 {
		statusBar = lipgloss.JoinHorizontal(lipgloss.Left, statusBar, strings.Repeat(" ", barWidth))
	}

	return lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBar)
}

func (m *inspectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	statusBarHeight := 1
	mainHeight := h - statusBarHeight

	leftPaneWidth := w / 3
	rightPaneWidth := w - leftPaneWidth

	// Always reserve space for 5 clients; scrolling kicks in for larger lists.
	// The DefaultDelegate renders each item at ~2 lines (title + cursor padding).
	clientsHeight := 5*2 + 2 // 5 items × 2 lines each + header chrome

	sHeight := mainHeight - clientsHeight - 8

	// Ensure Gaia Secret Viewer pane height is never smaller than clients pane height
	if sHeight < clientsHeight {
		// Calculate total available height for both panes
		totalAvailable := mainHeight - 8
		if totalAvailable > 0 {
			// Split height evenly, defaulting extra pixel to secrets
			clientsHeight = totalAvailable / 2
			sHeight = totalAvailable - clientsHeight
		} else {
			clientsHeight = 1
			sHeight = 1
		}
	}

	m.clientsList.SetSize(leftPaneWidth-4, clientsHeight)
	m.secretsList.SetSize(leftPaneWidth-4, sHeight)

	m.viewport.Width = rightPaneWidth - 4
	m.viewport.Height = mainHeight - 2
}
