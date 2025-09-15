package tui

import (
	"fmt"
	"os"
	"time"

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
		return m, nil

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
		return updateMainMenu(m, msg)
	case dataManagement:
		return updateDataManagement(m, msg)
	case certManagement:
		return updateCertManagement(m, msg)
	case addRecord:
		return updateAddRecord(m, msg)
	case createCerts:
		return updateCreateCerts(m, msg)
	case registerClient:
		return updateRegisterClient(m, msg)
	case listRecords: // New case
		return m.updateListRecords(msg)
	}

	return m, nil
}

// updateMainMenu handles all updates for the main menu screen.
func updateMainMenu(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
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
func updateDataManagement(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			selected := m.dataMenu.SelectedItem().(menuItem)
			switch selected.title {
			case "Add New Record":
				m.daemonStatus = "Loading clients..."
				return m, fetchClientsCmd(m.config)
			case "List All Records":
				m.activeScreen = listRecords // Navigate to the new screen
				return m, nil
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
			m.daemonStatus = fmt.Sprintf("Error loading clients: %v", msg.err)
			return m, nil
		}
		m.clients = msg.clients
		m.addRecordFormModel = newAddRecordFormModel(m.clients, m.namespaces)
		m.activeScreen = addRecord
		m.daemonStatus = "Enter new record details."
		return m, m.addRecordFormModel.Init()

	case NamespacesReadyMsg:
		m.namespaces = msg
		m.addRecordFormModel = newAddRecordFormModel(m.clients, m.namespaces)
		m.activeScreen = addRecord
		return m, m.addRecordFormModel.Init()
	}
	m.dataMenu, cmd = m.dataMenu.Update(msg)
	return m, cmd
}

// updateCertManagement handles updates for the certificate management screen.
func updateCertManagement(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
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

// updateAddRecord handles updates for the 'Add Record' form screen.
func updateAddRecord(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case AddRecordMsg:
		m.activeScreen = dataManagement
		m.daemonStatus = "Adding new record..."
		// Call the command to make the actual RPC call.
		return m, addRecordToDaemonCmd(m.config, msg.ClientName, msg.Namespace, msg.Key, msg.Value)
	case recordAddResultMsg:
		// Handle the result of the RPC call.
		m.activeScreen = dataManagement
		if msg.err != nil {
			m.daemonStatus = fmt.Sprintf("Error adding record: %v", msg.err)
		} else {
			m.daemonStatus = "Record added successfully!"
		}
		return m, nil
	case BackMsg:
		m.activeScreen = dataManagement
		return m, nil
	}

	if _, ok := msg.(BackMsg); ok {
		m.activeScreen = dataManagement
		return m, nil
	}
	if addMsg, ok := msg.(AddRecordMsg); ok {
		m.activeScreen = dataManagement
		m.daemonStatus = "Adding new record..."
		return m, addRecordToDaemon(addMsg.Namespace, addMsg.Key, addMsg.Value)
	}

	updatedModel, cmd := m.addRecordFormModel.Update(msg)
	m.addRecordFormModel = updatedModel.(*addRecordFormModel)
	return m, cmd
}

// updateCreateCerts handles updates for the 'Create Certificates' form screen.
func updateCreateCerts(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
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
func updateRegisterClient(m *model, msg tea.Msg) (tea.Model, tea.Cmd) {
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
