package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/certs"
)

func (m *model) Init() tea.Cmd {
	// Rebuild menu immediately with empty status (will be updated when status arrives)
	m.rebuildMainMenu()
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
		// Check status and schedule next tick
		return m, tea.Batch(
			checkStatusCmd(m.config),
			tea.Tick(m.config.GaiaTuiTickInterval, func(t time.Time) tea.Msg {
				return t
			}),
		)
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
		}
	case statusUpdatedMsg:
		// Set status - when err is present, status is already "offline"
		m.daemonStatus = msg.status
		// Rebuild main menu based on daemon state (locked/offline/unlocked)
		m.rebuildMainMenu()
		return m, nil
	case backToDataManagementMsg:
		m.activeScreen = dataManagement
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
	case unlockScreen:
		return m.updateUnlockScreen(msg)
	}

	return m, nil
}

// updateMainMenu handles all updates for the main menu screen.
func (m *model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		selected := m.mainMenu.SelectedItem().(menuItem)
		switch selected.title {
		case "Unlock Gaia":
			m.unlockFormModel = newUnlockFormModel()
			m.activeScreen = unlockScreen
			m.statusMessage = ""
			return m, m.unlockFormModel.Init()
		case "Manage Data", "Manage Data (Locked)", "Manage Data (Offline)":
			if isDaemonLocked(m.daemonStatus) {
				m.statusMessage = "⚠️ Cannot access data - Gaia is locked. Please unlock first."
				return m, nil
			}
			if isOffline(m.daemonStatus) {
				m.statusMessage = "⚠️ Cannot access data - Daemon is not running. Please start the daemon."
				return m, nil
			}
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
				// Check if the daemon is locked or offline
				if isDaemonLocked(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot add records - Gaia is locked. Please unlock first."
					m.activeScreen = mainMenu
					return m, nil
				}
				if isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot add records - Daemon is not running. Please start the daemon."
					m.activeScreen = mainMenu
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				// Fire the command to fetch the list of clients from the daemon.
				return m, fetchClientsCmd(m.config)

			case "List All Records":
				// Check if the daemon is locked or offline
				if isDaemonLocked(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot list records - Gaia is locked. Please unlock first."
					m.activeScreen = mainMenu
					return m, nil
				}
				if isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot list records - Daemon is not running. Please start the daemon."
					m.activeScreen = mainMenu
					return m, nil
				}
				m.activeScreen = listRecords
				return m, m.inspector.Init()

			case "Back":
				m.activeScreen = mainMenu
			}
		}

	case clientsLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Error loading clients: %v", msg.err)
			return m, nil
		}
		m.clients = make([]string, len(msg.clients))
		for i, c := range msg.clients {
			m.clients[i] = c.Name
		}
		m.addRecordFormModel = newAddRecordFormModel(m.clients)
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
		// Handle the escape key to exit the form (only 'esc', not 'b' since the form needs input)
		if msg.String() == "esc" {
			m.activeScreen = dataManagement
			m.statusMessage = ""
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
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			selected := m.certMenu.SelectedItem().(menuItem)
			switch selected.title {
			case "Create New Certificates":
				m.activeScreen = createCerts
				return m, m.certForm.Init()
			case "Register Client":
				m.registerClientFormModel = newRegisterClientFormModel()
				m.activeScreen = registerClient
				m.statusMessage = ""
				return m, m.registerClientFormModel.Init()
			case "List Existing Certificates":
				// TODO: Implement list functionality
			case "Back":
				m.activeScreen = mainMenu
			}
		case "b", "esc":
			m.activeScreen = mainMenu
			return m, nil
		}
	}
	m.certMenu, cmd = m.certMenu.Update(msg)
	return m, cmd
}

// updateCreateCerts handles updates for the 'Create Certificates' form screen.
func (m *model) updateCreateCerts(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle escape key to go back (for forms, only use esc, not 'b')
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		m.activeScreen = certManagement
		m.statusMessage = ""
		return m, nil
	}

	updatedForm, cmd := m.certForm.Update(msg)
	m.certForm = updatedForm.(*huh.Form)

	if m.certForm.State == huh.StateCompleted {
		outPath := m.certForm.GetString("outputPath")
		caName := m.certForm.GetString("caName")
		serverName := m.certForm.GetString("serverName")
		clientName := m.certForm.GetString("clientName")

		cfg := *m.config
		cfg.TLS.CertsDirectory = outPath

		var err error
		if err = certs.GenerateCA(&cfg, caName); err != nil {
			err = fmt.Errorf("generating CA failed: %w", err)
		} else if err = certs.GenerateServerCertificate(&cfg, serverName); err != nil {
			err = fmt.Errorf("generating server certificate failed: %w", err)
		} else if err = certs.GenerateClientCertificate(&cfg, clientName); err != nil {
			err = fmt.Errorf("generating client certificate failed: %w", err)
		}

		if err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error generating certificates: %v", err)
		} else {
			m.statusMessage = fmt.Sprintf("✓ Certificates generated successfully in %s/", outPath)
		}

		m.activeScreen = certManagement
		return m, nil
	}
	return m, cmd
}

// updateRegisterClient handles updates for the 'Register Client' form screen.
func (m *model) updateRegisterClient(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BackMsg:
		m.activeScreen = certManagement
		m.statusMessage = ""
		return m, nil
	case RegisterClientMsg:
		m.statusMessage = fmt.Sprintf("Registering client '%s'...", msg.ClientName)
		return m, registerClientCmd(m.config, msg.ClientName)
	case clientRegisteredMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error registering client: %v", msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("✓ Client '%s' registered successfully!\nCert: %s\nKey: %s",
				msg.clientName, msg.certPath, msg.keyPath)
		}
		m.activeScreen = certManagement
		return m, nil
	}

	updatedModel, cmd := m.registerClientFormModel.Update(msg)
	m.registerClientFormModel = updatedModel.(*registerClientFormModel)
	return m, cmd
}

// updateUnlockScreen handles updates for the unlock screen.
func (m *model) updateUnlockScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case BackMsg:
		m.activeScreen = mainMenu
		m.statusMessage = ""
		return m, nil
	case UnlockMsg:
		m.statusMessage = "Unlocking Gaia..."
		return m, unlockDaemonCmd(m.config, msg.Passphrase)
	case unlockResultMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error unlocking: %v", msg.err)
			// Stay on unlock screen to allow retry
			m.unlockFormModel = newUnlockFormModel()
			return m, m.unlockFormModel.Init()
		}
		if !msg.success {
			m.statusMessage = "❌ Unlock failed: Invalid passphrase"
			// Stay on unlock screen to allow retry
			m.unlockFormModel = newUnlockFormModel()
			return m, m.unlockFormModel.Init()
		}
		m.statusMessage = "✓ Gaia unlocked successfully!"
		m.activeScreen = mainMenu
		// Trigger a status check to update the daemon status
		return m, checkStatusCmd(m.config)
	}

	updatedModel, cmd := m.unlockFormModel.Update(msg)
	m.unlockFormModel = updatedModel.(*unlockFormModel)
	return m, cmd
}

// rebuildMainMenu rebuilds the main menu items based on the daemon state.
func (m *model) rebuildMainMenu() {
	// Common menu items that appear in all states
	certItem := menuItem{"Manage Certificates", "View and manage your certificates"}
	quitItem := menuItem{"Quit", "Exit the Gaia application (q)"}

	var items []list.Item

	switch {
	case isDaemonLocked(m.daemonStatus):
		// When locked, show Unlock Gaia first
		items = []list.Item{
			menuItem{"Unlock Gaia", "Unlock the daemon to access secrets"},
			menuItem{"Manage Data (Locked)", "⚠️ Requires unlock - Add, view, or delete secret records"},
			certItem,
			quitItem,
		}
	case isOffline(m.daemonStatus):
		// When offline/stopped/starting, show a disabled menu
		items = []list.Item{
			menuItem{"Manage Data (Offline)", "⚠️ Daemon not running - Cannot access secret records"},
			certItem,
			quitItem,
		}
	default:
		// When unlocked or any other state, show a normal menu
		items = []list.Item{
			menuItem{"Manage Data", "Add, view, or delete secret records"},
			certItem,
			quitItem,
		}
	}

	m.mainMenu.SetItems(items)
}
