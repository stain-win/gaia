package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/certs"
	"github.com/stain-win/gaia/apps/gaia/daemon"
)

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		checkStatusCmd(m.config),
		tea.Tick(m.config.GaiaTuiTickInterval, func(t time.Time) tea.Msg {
			return t
		}),
	)
}

// Update is called when a message is received.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global handling for messages that apply to all screens
	switch msg := msg.(type) {
	case time.Time:
		return m, checkStatusCmd(m.config)
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(8, 2).GetFrameSize()
		m.mainMenu.SetSize(msg.Width-h, min(len(m.mainMenu.Items())*5, msg.Height-v))
		m.dataMenu.SetSize(msg.Width-h, min(len(m.dataMenu.Items())*5, msg.Height-v))
		m.certMenu.SetSize(msg.Width-h, min(len(m.certMenu.Items())*5, msg.Height-v))

		m.width = msg.Width
		m.height = msg.Height

		if m.inspector != nil {
			m.inspector.SetSize(msg.Width-h, msg.Height-v)
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, keys.Back):
			if m.activeScreen != mainMenu {
				m.activeScreen = mainMenu
				return m, nil
			}
		}
	case statusUpdatedMsg:
		if msg.err != nil {
			m.daemonStatus = fmt.Sprintf("%s - %s", msg.status, "could not connect to daemon")
		} else {
			m.daemonStatus = msg.status
		}
		return m, nil
	}

	// Screen-specific updates
	switch m.activeScreen {
	case mainMenu:
		return m.updateMainMenu(msg)
	case dataManagement:
		return m.updateDataManagement(msg)
	case certManagement:
		return m.updateCertManagement(msg)
	case addRecord:
		return m.updateAddRecord(msg)
	case createCerts:
		return m.updateCreateCerts(msg)
	case registerClient:
		return m.updateRegisterClient(msg)
	case listRecords:
		var cmd tea.Cmd
		m.inspector, cmd = m.inspector.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateMainMenu handles all updates for the main menu screen.
func (m *model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		selected := m.mainMenu.SelectedItem().(menuItem)
		switch selected.title {
		case "Manage Data":
			m.activeScreen = dataManagement
		case "Manage Certificates":
			m.activeScreen = certManagement
		case "Quit":
			m.quitting = true
			return m, tea.Quit
		}
	}
	m.mainMenu, cmd = m.mainMenu.Update(msg)
	return m, cmd
}

// updateDataManagement handles updates for the data management screen.
func (m *model) updateDataManagement(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Back) {
			m.activeScreen = mainMenu
			return m, nil
		}
		if key.Matches(msg, keys.Enter) {
			selected := m.dataMenu.SelectedItem().(menuItem)
			switch selected.title {
			case "Add New Record":
				m.statusMessage = "Loading clients..."
				// Fire the command to fetch the list of clients from the daemon.
				return m, fetchClientsCmd(m.config)

			case "List All Records":
				m.activeScreen = listRecords
				return m, m.inspector.Init()

			case "Back":
				m.activeScreen = mainMenu
			}
		}
	case daemon.StatusMsg:
		if msg.Err != nil || msg.Status != "running" {
			m.daemonStatus = fmt.Sprintf("Error: Daemon not running (Status: %s)", msg.Status)
			return m, nil
		}
		m.daemonStatus = "Daemon running. Fetching namespaces..."
		return m, mockListNamespaces()

	case clientsLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error loading clients: %v", msg.err)
			return m, nil
		}
		m.clients = msg.clients
		m.addRecordFormModel = newAddRecordFormModel(m.clients, m.namespaces)
		m.activeScreen = addRecord
		m.statusMessage = "Enter new record details."
		return m, m.addRecordFormModel.Init()
	}
	m.dataMenu, cmd = m.dataMenu.Update(msg)
	return m, cmd
}

func (m *model) updateAddRecord(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case AddRecordMsg:
		m.statusMessage = "Adding record..."
		// Fire the command to add the secret via gRPC.
		return m, addRecordToDaemonCmd(m.config, msg.ClientName, msg.Namespace, msg.Key, msg.Value)
	// This new case handles the result of the addRecordCmd.
	case recordAddedMsg:
		if msg.err != nil {

			m.statusMessage = fmt.Sprintf("Error adding record: %v", msg.err)
		} else {
			m.statusMessage = "Record added successfully!"
		}
		// Go back to the data management menu.
		m.activeScreen = dataManagement
		return m, nil

	case tea.KeyMsg:
		// Handle the escape key specifically to exit the form.
		if key.Matches(msg, keys.Back) { // Using the 'Back' keybinding
			m.activeScreen = dataManagement
			return m, nil
		}
	}

	var updatedForm tea.Model
	updatedForm, cmd = m.addRecordFormModel.Update(msg)
	m.addRecordFormModel = updatedForm.(*addRecordFormModel)

	return m, cmd
}

// updateCertManagement handles updates for the certificate management screen.
func (m *model) updateCertManagement(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
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
		case "Back":
			m.activeScreen = mainMenu
		}
	}
	m.certMenu, cmd = m.certMenu.Update(msg)
	return m, cmd
}

// updateCreateCerts handles updates for the 'Create Certificates' form screen.
func (m *model) updateCreateCerts(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedForm, cmd := m.certForm.Update(msg)
	m.certForm = updatedForm.(*huh.Form)

	if m.certForm.State == huh.StateCompleted {
		err := certs.GenerateTLSCertificates(
			m.certForm.GetString("outputPath"),
			m.certForm.GetString("caName"),
			m.certForm.GetString("serverName"),
			m.certForm.GetString("clientName"),
		)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error generating certificates: %v\n", err)
		} else {
			fmt.Println("Certificates generated successfully!")
		}
		m.activeScreen = certManagement
		return m, tea.ClearScreen
	}
	return m, cmd
}

// updateRegisterClient handles updates for the 'Register Client' form screen.
func (m *model) updateRegisterClient(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(BackMsg); ok {
		m.activeScreen = certManagement
		return m, nil
	}
	if regMsg, ok := msg.(RegisterClientMsg); ok {
		// TODO: Call gRPC client to register the client.
		fmt.Printf("Received RegisterClientMsg: ClientName=%s\n", regMsg.ClientName)
		m.activeScreen = certManagement
		return m, nil
	}

	updatedModel, cmd := m.registerClientFormModel.Update(msg)
	m.registerClientFormModel = updatedModel.(*registerClientFormModel)
	return m, cmd
}
