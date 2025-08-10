package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	bubblesTable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/tui/table"
)

// listRecordsModel holds the state for the screen that lists all records.
type listRecordsModel struct {
	namespaces list.Model
	secrets    table.Model
	help       help.Model
	keys       keyMap
	activePane int // 0 for namespaces, 1 for secrets
	width      int
	height     int
}

type namespaceItem struct {
	name string
	desc string
}

func (n namespaceItem) FilterValue() string { return n.name }
func (n namespaceItem) Title() string       { return n.name }
func (n namespaceItem) Description() string { return n.desc }

// keyMap defines the keybindings for the list records screen.
type keyMap struct {
	Tab    key.Binding
	Esc    key.Binding
	Quit   key.Binding
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Add    key.Binding
	Delete key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Esc, k.Add, k.Delete, k.Enter}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.Esc},
		{k.Add, k.Delete, k.Enter, k.Quit},
	}
}

var keys = keyMap{
	Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
	Esc:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/edit")),
	Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add record")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete record")),
}

// newListRecordsModel initializes the model for the list records screen.
func newListRecordsModel() listRecordsModel {
	// For now, we'll use mock data. This will be replaced by a gRPC call.
	namespaceItems := []list.Item{
		namespaceItem{"common", "Common namespace for shared secrets"},
		namespaceItem{"client-app-a", "Namespace for client application A"},
		namespaceItem{"client-app-b", "Namespace for client application B"},
	}

	nsList := list.New(namespaceItems, list.NewDefaultDelegate(), 0, 0)
	nsList.SetShowHelp(false)
	//nsList.SetShowTitle(false)
	//nsList.SetShowStatusBar(false)
	nsList.SetShowPagination(false)

	// Define the columns for our secrets table.
	columns := []bubblesTable.Column{
		{Title: "Key", Width: 20},
		{Title: "Value", Width: 40},
	}

	// Mock rows for the 'common' namespace.
	rows := []bubblesTable.Row{
		{"api_key_service_x", "****************"},
		{"database_url", "postgres://..."},
		{"stripe_api_key", "sk_test_..."},
	}

	secretsTable := table.New(
		bubblesTable.WithColumns(columns),
		bubblesTable.WithRows(rows),
		bubblesTable.WithFocused(true),
		bubblesTable.WithHeight(10),
	)

	return listRecordsModel{
		namespaces: nsList,
		secrets:    secretsTable,
		help:       help.New(),
		keys:       keys,
		activePane: 0,
	}
}

// updateListRecords handles messages for the list records screen.
func (m *model) updateListRecords(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.listRecords.width = msg.Width
		m.listRecords.height = msg.Height
		m.listRecords.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.listRecords.keys.Tab):
			m.listRecords.activePane = (m.listRecords.activePane + 1) % 2
		case key.Matches(msg, m.listRecords.keys.Esc):
			m.activeScreen = dataManagement
			return m, nil
		}
	}

	if m.listRecords.activePane == 0 {
		m.listRecords.namespaces, cmd = m.listRecords.namespaces.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.listRecords.secrets, cmd = m.listRecords.secrets.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// viewListRecords renders the list records screen.
func (m *model) viewListRecords() string {
	// Define styles for active and inactive panes
	inactivePaneStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, false).
		BorderTop(true).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)

	activePaneStyle := inactivePaneStyle.
		Border(lipgloss.ThickBorder(), false, false, false, false).
		BorderTop(true).
		BorderForeground(lipgloss.Color("226"))

	// Calculate pane widths and heights
	helpHeight := lipgloss.Height(m.listRecords.help.View(m.listRecords.keys))
	verticalMargin := lipgloss.Height(activePaneStyle.Render(""))
	paneHeight := m.height - helpHeight - verticalMargin - 4 // Adjust for logo and margins

	namespaceWidth := m.listRecords.width / 2
	tableWidth := m.listRecords.width - namespaceWidth - 8 // Adjust for padding/margins

	m.listRecords.namespaces.SetHeight(paneHeight)
	m.listRecords.secrets.SetHeight(paneHeight)
	m.listRecords.secrets.SetWidth(tableWidth)

	var nsStyle, tblStyle lipgloss.Style
	if m.listRecords.activePane == 0 {
		nsStyle = activePaneStyle
		//tblStyle = inactivePaneStyle
	} else {
		nsStyle = inactivePaneStyle
		//tblStyle = activePaneStyle
	}

	// Render panes with titles
	nsView := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Namespaces"),
		m.listRecords.namespaces.View(),
	)
	tblView := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Secrets"),
		m.listRecords.secrets.View(),
	)

	panes := lipgloss.JoinHorizontal(
		lipgloss.Top,
		nsStyle.Width(namespaceWidth).Render(nsView),
		tblStyle.Width(tableWidth).Render(tblView),
	)

	return lipgloss.JoinVertical(lipgloss.Left, panes, m.listRecords.help.View(m.listRecords.keys))
}
