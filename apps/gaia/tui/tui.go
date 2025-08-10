package tui

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
)

// Global ASCII art for the Gaia logo.
const gaiaLogo = `

      ___           ___                       ___     
     /\  \         /\  \          ___        /\  \    
    /::\  \       /::\  \        /\  \      /::\  \   
   /:/\:\  \     /:/\:\  \       \:\  \    /:/\:\  \  
  /:/  \:\  \   /::\~\:\  \      /::\__\  /::\~\:\  \ 
 /:/__/_\:\__\ /:/\:\ \:\__\  __/:/\/__/ /:/\:\ \:\__\
 \:\  /\ \/__/ \/__\:\/:/  / /\/:/  /    \/__\:\/:/  /
  \:\ \:\__\        \::/  /  \::/__/          \::/  / 
   \:\/:/  /        /:/  /    \:\__\          /:/  /  
    \::/  /        /:/  /      \/__/         /:/  /   
     \/__/         \/__/                     \/__/    

`

type screen int

const (
	mainMenu screen = iota
	dataManagement
	addRecord
	certManagement
	createCerts
	registerClient
)

// A custom item type for our list.
type menuItem struct {
	title string
	desc  string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// The `model` now holds the interactive list for the main menu.
type model struct {
	activeScreen            screen
	mainMenu                list.Model
	dataMenu                list.Model
	certMenu                list.Model               // New list for certificate management
	addRecordFormModel      *addRecordFormModel      // Embedded form model pointer
	registerClientFormModel *registerClientFormModel // New model for client registration
	quitting                bool
	// Certificate generation form fields
	certForm   *huh.Form
	caName     string
	serverName string
	clientName string
	outputPath string
	// TUI layout dimensions
	width  int
	height int
	// Data storage for the TUI
	namespaces    []string
	statusMessage string
	// The application's configuration
	config *config.Config
}

// These are the new list items, which can be selected and navigated.
var menuItems = []list.Item{
	menuItem{"Manage Data", "Add, view, or delete secret records"},
	menuItem{"Manage Certificates", "View and manage your certificates"},
	menuItem{"Quit", "Exit the Gaia application (q)"},
}

var dataMenuItems = []list.Item{
	menuItem{"Add New Record", "Add a new secret record to Gaia"},
	menuItem{"List All Records", "View all records currently in the database"},
	menuItem{"Back", "Return to the main menu (b)"},
}

var certMenuItems = []list.Item{
	menuItem{"Create New Certificates", "Generate a new set of mTLS certificates"},
	menuItem{"Register Client", "Register a new client for namespacing and mTLS"},
	menuItem{"List Existing Certificates", "View all certificates known to Gaia"},
	menuItem{"Back", "Return to the main menu (b)"},
}

// A custom list delegate to render our menu items with styles.
type menuDelegate struct{}

func (d menuDelegate) Height() int                         { return 1 }
func (d menuDelegate) Spacing() int                        { return 0 }
func (d menuDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d menuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	s := ""
	i, ok := item.(menuItem)
	if !ok {
		return
	}

	// Apply styles for selected vs unselected items.
	if index == m.Index() {
		s = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#226CFF")).
			Background(lipgloss.Color("#5A5B69")).
			Bold(true).
			Render("> " + i.title)
	} else {
		s = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Render("  " + i.title)
	}
	_, err := fmt.Fprintf(w, s)
	if err != nil {
		return
	}
}

// BackMsg is a custom message to signal returning to the main menu.
type BackMsg struct{}

// ListNamespacesMsg is a custom message to trigger fetching namespaces from the daemon.
type ListNamespacesMsg struct{}

// NamespacesReadyMsg is a custom message for when namespaces are fetched.
type NamespacesReadyMsg []string

// A mock function to simulate fetching namespaces from the daemon.
func mockListNamespaces() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond) // Simulate network latency
		namespaces := []string{"common", "client-a", "client-b"}
		return NamespacesReadyMsg(namespaces)
	}
}

func addRecordToDaemon(namespace, key, value string) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder; in a real app, you'd make a gRPC call.
		fmt.Printf("Making gRPC call to add record: Namespace=%s, Key=%s, Value=%s\n", namespace, key, value)
		// Assume success for now
		return fmt.Errorf("record added successfully!")
	}
}

func initialModel(cfg *config.Config) model {
	// Initialize the list with the custom delegate and items.
	mainList := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	mainList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Main Menu")

	// Initialize the data management list
	dataList := list.New(dataMenuItems, list.NewDefaultDelegate(), 0, 0)
	dataList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Data Management")

	// Initialize the cert management list
	certList := list.New(certMenuItems, list.NewDefaultDelegate(), 0, 0)
	certList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Certificate Management")

	// Initialize the certificate generation form
	m := model{
		activeScreen:            mainMenu,
		mainMenu:                mainList,
		dataMenu:                dataList,
		certMenu:                certList,
		addRecordFormModel:      newAddRecordFormModel(nil),
		registerClientFormModel: newRegisterClientFormModel(), // Initialize the new form model
		statusMessage:           "",
		config:                  cfg,
	}

	m.certForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("caName").Title("CA Common Name").Value(&m.caName),
			huh.NewInput().Key("serverName").Title("Server Common Name").Value(&m.serverName),
			huh.NewInput().Key("clientName").Title("Client Common Name").Value(&m.clientName),
			huh.NewInput().Key("outputPath").Title("Output Directory").Placeholder("./certs").Value(&m.outputPath),
		),
	)

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(8, 2).GetFrameSize()
		m.mainMenu.SetSize(msg.Width-h, msg.Height-v)
		m.dataMenu.SetSize(msg.Width-h, msg.Height-v)
		m.certMenu.SetSize(msg.Width-h, msg.Height-v)
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if m.activeScreen != mainMenu {
				m.activeScreen = mainMenu
				return m, nil
			}
		case "enter":
			switch m.activeScreen {
			case mainMenu:
				selected := m.mainMenu.SelectedItem().(menuItem)
				switch selected.title {
				case "Manage Data":
					m.activeScreen = dataManagement
					return m, nil
				case "Manage Certificates":
					m.activeScreen = certManagement
					return m, nil
				case "Quit":
					m.quitting = true
					return m, tea.Quit
				}
			case dataManagement:
				selected := m.dataMenu.SelectedItem().(menuItem)
				switch selected.title {
				case "Add New Record":
					// Check daemon status first, passing the config
					return m, daemon.CheckDaemonStatus(m.config)
				case "List All Records":
					// TODO: Implement list functionality
					return m, nil
				case "Back":
					m.activeScreen = mainMenu
					return m, nil
				}
			case certManagement:
				selected := m.certMenu.SelectedItem().(menuItem)
				switch selected.title {
				case "Create New Certificates":
					m.activeScreen = createCerts
					return m, m.certForm.Init()
				case "Register Client":
					m.activeScreen = registerClient
					return m, m.registerClientFormModel.Init()
				case "List Existing Certificates":
					// TODO: Implement list functionality
					return m, nil
				case "Back":
					m.activeScreen = mainMenu
					return m, nil
				}
			}
		}
	case AddRecordMsg:
		m.activeScreen = dataManagement
		// A message from the Add Record form tells us to process the new secret.
		m.statusMessage = "Adding new record..."
		// Call the gRPC client to add the secret to the daemon.
		return m, addRecordToDaemon(msg.Namespace, msg.Key, msg.Value)

	case RegisterClientMsg:
		// A message from the Register Client form tells us to process the new client.
		// TODO: Call the gRPC client to register the client.
		fmt.Printf("Received RegisterClientMsg: ClientName=%s\n", msg.ClientName)
		m.activeScreen = certManagement
		return m, nil

	case NamespacesReadyMsg:
		m.namespaces = msg
		m.addRecordFormModel = newAddRecordFormModel(m.namespaces)
		m.activeScreen = addRecord
		return m, m.addRecordFormModel.Init()

	case daemon.DaemonStatusMsg:
		if msg.Err != nil || msg.Status != "running" {
			m.statusMessage = fmt.Sprintf("Error: Daemon is not running. Status: %s. Run 'gaia start' first.", msg.Status)
			return m, nil
		}
		// Daemon is running, proceed to fetch namespaces
		m.statusMessage = "Daemon is running. Fetching namespaces..."
		return m, mockListNamespaces()

	case BackMsg:
		if m.activeScreen == addRecord || m.activeScreen == createCerts || m.activeScreen == registerClient {
			m.activeScreen = certManagement
			return m, nil
		}
		if m.activeScreen == certManagement || m.activeScreen == dataManagement {
			m.activeScreen = mainMenu
			return m, nil
		}
		return m, nil
	}

	// Update the appropriate sub-model based on the active screen.
	switch m.activeScreen {
	case mainMenu:
		m.mainMenu, cmd = m.mainMenu.Update(msg)
	case dataManagement:
		m.dataMenu, cmd = m.dataMenu.Update(msg)
	case certManagement:
		m.certMenu, cmd = m.certMenu.Update(msg)
	case addRecord:
		updatedModel, formCmd := m.addRecordFormModel.Update(msg)
		m.addRecordFormModel = updatedModel.(*addRecordFormModel)
		cmd = formCmd
	case createCerts:
		updatedModel, formCmd := m.certForm.Update(msg)
		m.certForm = updatedModel.(*huh.Form)
		cmd = formCmd
		if m.certForm.State == huh.StateCompleted {
			caName := m.certForm.GetString("caName")
			serverName := m.certForm.GetString("serverName")
			clientName := m.certForm.GetString("clientName")
			outputPath := m.certForm.GetString("outputPath")

			err := certs.GenerateTLSCertificates(outputPath, caName, serverName, clientName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating certificates: %v\n", err)
			} else {
				fmt.Println("Certificates generated successfully!")
			}
			m.activeScreen = certManagement
			return m, tea.ClearScreen
		}
	case registerClient:
		updatedModel, formCmd := m.registerClientFormModel.Update(msg)
		m.registerClientFormModel = updatedModel.(*registerClientFormModel)
		cmd = formCmd
	}

	return m, cmd
}

func (m model) View() string {
	logo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6A5ACD")).
		Align(lipgloss.Center).
		Render(gaiaLogo)

	var screenView string
	switch m.activeScreen {
	case mainMenu:
		screenView = m.mainMenu.View()
	case dataManagement:
		screenView = m.dataMenu.View()
	case addRecord:
		screenView = m.addRecordFormModel.View() // Render the form view
	case certManagement:
		screenView = m.certMenu.View()
	case createCerts:
		screenView = m.certForm.View()
	case registerClient:
		screenView = m.registerClientFormModel.View()
	}

	// Center the combined logo and screenView horizontally
	content := lipgloss.JoinVertical(lipgloss.Center, logo, screenView)
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(content)
}

func Run(cfg *config.Config) error {
	p := tea.NewProgram(initialModel(cfg), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
