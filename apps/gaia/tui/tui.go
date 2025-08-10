package tui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/certs"
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
	addRecord
	certManagement
	createCerts
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
	activeScreen       screen
	mainMenu           list.Model
	certMenu           list.Model          // New list for certificate management
	addRecordFormModel *addRecordFormModel // Embedded form model pointer
	quitting           bool
	// Certificate generation form fields
	certForm   *huh.Form
	caName     string
	serverName string
	clientName string
	outputPath string
	// TUI layout dimensions
	width  int
	height int
}

// These are the new list items, which can be selected and navigated.
var menuItems = []list.Item{
	menuItem{"Add New Secret", "Add a new secret record to Gaia"},
	menuItem{"Manage Certificates", "View and manage your certificates"},
	menuItem{"Quit", "Exit the Gaia application (q)"},
}

var certMenuItems = []list.Item{
	menuItem{"Create New Certificates", "Generate a new set of mTLS certificates"},
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

func initialModel() model {
	// Initialize the list with the custom delegate and items.
	mainList := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	mainList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Main Menu")

	// Initialize the cert management list
	certList := list.New(certMenuItems, list.NewDefaultDelegate(), 0, 0)
	certList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Certificate Management")

	// Initialize the model with a temporary struct to bind variables.
	// This fixes the 'undefined' errors for certForm fields.
	m := model{
		activeScreen:       mainMenu,
		mainMenu:           mainList,
		certMenu:           certList,
		addRecordFormModel: newAddRecordFormModel(), // Initialize the new form model
	}

	// Initialize the certificate generation form
	// The variables are now part of the model struct.
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
				case "Add New Secret":
					m.activeScreen = addRecord
					return m, m.addRecordFormModel.Init()
				case "Manage Certificates":
					m.activeScreen = certManagement
					return m, nil
				case "Quit":
					m.quitting = true
					return m, tea.Quit
				}
			case certManagement:
				selected := m.certMenu.SelectedItem().(menuItem)
				switch selected.title {
				case "Create New Certificates":
					m.activeScreen = createCerts
					return m, m.certForm.Init()
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
		// A message from the Add Record form tells us to process the new secret.
		// TODO: Call the gRPC client to add the secret to the daemon.
		fmt.Printf("Received AddRecordMsg: Namespace=%s, Key=%s, Value=%s\n", msg.Namespace, msg.Key, msg.Value)
		m.activeScreen = mainMenu
		return m, nil

	case BackMsg:
		m.activeScreen = mainMenu
		return m, nil
	}

	// Update the appropriate sub-model based on the active screen.
	switch m.activeScreen {
	case mainMenu:
		m.mainMenu, cmd = m.mainMenu.Update(msg)
	case certManagement:
		m.certMenu, cmd = m.certMenu.Update(msg)
	case addRecord:
		updatedModel, formCmd := m.addRecordFormModel.Update(msg)
		m.addRecordFormModel = updatedModel.(*addRecordFormModel)
		cmd = formCmd
	case createCerts:
		// The cert form will handle its own updates
		updatedModel, formCmd := m.certForm.Update(msg)
		m.certForm = updatedModel.(*huh.Form)
		cmd = formCmd
		if m.certForm.State == huh.StateCompleted {
			// Get values from the form and generate certs
			caName := m.certForm.GetString("caName")
			serverName := m.certForm.GetString("serverName")
			clientName := m.certForm.GetString("clientName")
			outputPath := m.certForm.GetString("outputPath")

			err := certs.GenerateTLSCertificates(outputPath, caName, serverName, clientName)
			if err != nil {
				// TODO: Handle error message in TUI
				fmt.Fprintf(os.Stderr, "Error generating certificates: %v\n", err)
			} else {
				// TODO: Show success message
				fmt.Println("Certificates generated successfully!")
			}
			m.activeScreen = certManagement
			return m, tea.ClearScreen
		}
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
		screenView = logo + m.mainMenu.View()
	case addRecord:
		screenView = m.addRecordFormModel.View() // Render the form view
	case certManagement:
		screenView = m.certMenu.View()
	case createCerts:
		screenView = m.certForm.View()
	}

	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(
			lipgloss.Place(
				m.width,
				m.height,
				lipgloss.Center,
				lipgloss.Center,
				screenView,
			),
		)
}

//func certManagementView(m model) string {
//	s := "Certificate Management\n"
//	for _, c := range m.certs {
//		s += "- " + c + "\n"
//	}
//	s += "\n[b] Back to menu\n"
//	return s
//}

// Mock gRPC calls
func mockAddSecret(key, value, desc string) {
	fmt.Printf("[gRPC] AddSecret called: key=%s, value=%s, desc=%s\n", key, value, desc)
}

func mockListCerts() []string {
	return []string{"CertA (valid)", "CertB (revoked)", "CertC (valid)"}
}

func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
