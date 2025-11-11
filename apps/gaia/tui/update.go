package tui

import (
	"fmt"
	"strings"
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
		m.accessMenu.SetSize(msg.Width-h, min(len(m.accessMenu.Items())*5, msg.Height-v))
		m.certMenu.SetSize(msg.Width-h, min(len(m.certMenu.Items())*5, msg.Height-v))
		m.policyMenu.SetSize(msg.Width-h, min(len(m.policyMenu.Items())*5, msg.Height-v))

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
	case accessManagement:
		return m.updateAccessManagement(msg)
	case certManagement:
		return m.updateCertManagement(msg)
	case policyManagement:
		return m.updatePolicyManagement(msg)
	case listPolicies:
		return m.updateListPolicies(msg)
	case viewPolicy:
		return m.updateViewPolicy(msg)
	case selectPolicyClient:
		return m.updateSelectPolicyClient(msg)
	case createPolicy:
		return m.updateCreatePolicy(msg)
	case editPolicy:
		return m.updateEditPolicy(msg)
	case deletePolicy:
		return m.updateDeletePolicy(msg)
	case policyExport:
		return m.updatePolicyExport(msg)
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
		case "Manage Access":
			m.activeScreen = accessManagement
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
				m.activeScreen = accessManagement
			}
		case "b", "esc":
			m.activeScreen = accessManagement
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
	accessItem := menuItem{"Manage Access", "Manage certificates and authorization policies"}
	quitItem := menuItem{"Quit", "Exit the Gaia application (q)"}

	var items []list.Item

	switch {
	case isDaemonLocked(m.daemonStatus):
		// When locked, show Unlock Gaia first
		items = []list.Item{
			menuItem{"Unlock Gaia", "Unlock the daemon to access secrets"},
			menuItem{"Manage Data (Locked)", "⚠️ Requires unlock - Add, view, or delete secret records"},
			accessItem,
			quitItem,
		}
	case isOffline(m.daemonStatus):
		// When offline/stopped/starting, show a disabled menu
		items = []list.Item{
			menuItem{"Manage Data (Offline)", "⚠️ Daemon not running - Cannot access secret records"},
			accessItem,
			quitItem,
		}
	default:
		// When unlocked or any other state, show a normal menu
		items = []list.Item{
			menuItem{"Manage Data", "Add, view, or delete secret records"},
			accessItem,
			quitItem,
		}
	}

	m.mainMenu.SetItems(items)
}

// updateAccessManagement handles updates for the access management menu screen.
func (m *model) updateAccessManagement(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			selected := m.accessMenu.SelectedItem().(menuItem)
			switch selected.title {
			case "Manage Certificates":
				m.activeScreen = certManagement
			case "Manage Policies":
				m.activeScreen = policyManagement
			case "Back":
				m.activeScreen = mainMenu
			}
		case "b", "esc":
			m.activeScreen = mainMenu
			return m, nil
		}
	}
	m.accessMenu, cmd = m.accessMenu.Update(msg)
	return m, cmd
}

// updatePolicyManagement handles updates for the policy management menu screen.
func (m *model) updatePolicyManagement(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			selected := m.policyMenu.SelectedItem().(menuItem)
			switch selected.title {
			case "List All Policies":
				if isDaemonLocked(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot access policies - Gaia is locked. Please unlock first."
					return m, nil
				}
				if isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot access policies - Daemon is not running."
					return m, nil
				}
				m.statusMessage = "Loading policies..."
				return m, fetchPoliciesCmd(m.config)
			case "View Policy":
				if isDaemonLocked(m.daemonStatus) || isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot access policies - Daemon is not available."
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				m.activeScreen = selectPolicyClient
				return m, fetchClientsForPolicyCmd(m.config, "view")
			case "Create New Policy":
				if isDaemonLocked(m.daemonStatus) || isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot create policy - Daemon is not available."
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				m.activeScreen = selectPolicyClient
				return m, fetchClientsForPolicyCmd(m.config, "create")
			case "Edit Policy":
				if isDaemonLocked(m.daemonStatus) || isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot edit policy - Daemon is not available."
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				m.activeScreen = selectPolicyClient
				return m, fetchClientsForPolicyCmd(m.config, "edit")
			case "Delete Policy":
				if isDaemonLocked(m.daemonStatus) || isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot delete policy - Daemon is not available."
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				m.activeScreen = selectPolicyClient
				return m, fetchClientsForPolicyCmd(m.config, "delete")
			case "Export Policy":
				if isDaemonLocked(m.daemonStatus) || isOffline(m.daemonStatus) {
					m.statusMessage = "⚠️ Cannot export policy - Daemon is not available."
					return m, nil
				}
				m.statusMessage = "Loading clients..."
				m.activeScreen = selectPolicyClient
				return m, fetchClientsForPolicyCmd(m.config, "export")
			case "Back":
				m.activeScreen = accessManagement
			}
		case "b", "esc":
			m.activeScreen = accessManagement
			return m, nil
		}
	}
	m.policyMenu, cmd = m.policyMenu.Update(msg)
	return m, cmd
}

// updateListPolicies handles updates for the list policies screen.
func (m *model) updateListPolicies(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "b", "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			return m, nil
		case "enter":
			if len(m.policies) > 0 {
				// TODO: Show policy details
				m.statusMessage = "View policy details - Coming soon"
			}
			return m, nil
		}
	case policiesLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading policies: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}
		m.policies = msg.policies
		m.activeScreen = listPolicies
		m.statusMessage = fmt.Sprintf("Loaded %d policies", len(m.policies))
		return m, nil
	}
	return m, nil
}

// updateViewPolicy handles updates for viewing a specific policy.
func (m *model) updateViewPolicy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "b", "esc":
			m.activeScreen = listPolicies
			m.statusMessage = ""
			return m, nil
		}

		// Pass viewport controls to the policy detail model
		if m.selectedPolicy != nil {
			m.selectedPolicy.viewport, _ = m.selectedPolicy.viewport.Update(msg)
		}
	case tea.WindowSizeMsg:
		if m.selectedPolicy != nil && !m.selectedPolicy.ready {
			m.selectedPolicy.setSize(msg.Width, msg.Height)
		}
	}
	return m, nil
}

// updateSelectPolicyClient handles client selection for policy operations
func (m *model) updateSelectPolicyClient(msg tea.Msg) (tea.Model, tea.Cmd) {
	var operation string

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "b", "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			m.selectedClientName = ""
			return m, nil
		case "enter":
			// Get selected client
			if len(m.clients) == 0 {
				return m, nil
			}

			// For now, select the first client (TODO: make this interactive)
			m.selectedClientName = m.clients[0]

			// Route based on stored operation (we need to track this)
			// For now, just load the policy
			m.statusMessage = fmt.Sprintf("Loading policy for '%s'...", m.selectedClientName)
			return m, fetchPolicyCmd(m.config, m.selectedClientName)
		}
	case clientsForPolicyMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading clients: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}

		// Store operation type and clients
		m.clients = msg.clients
		operation = msg.operation

		// If only one client, auto-select
		if len(m.clients) == 1 {
			m.selectedClientName = m.clients[0]

			// Route immediately based on operation
			switch operation {
			case "view", "edit", "delete", "export":
				m.statusMessage = fmt.Sprintf("Loading policy for '%s'...", m.selectedClientName)
				return m, fetchPolicyCmd(m.config, m.selectedClientName)
			case "create":
				// Initialize editor for new policy
				m.policyEditorModel = newPolicyEditorModel(m.selectedClientName, nil)
				if m.policyEditorModel != nil {
					m.policyEditorModel.form = m.policyEditorModel.Init()
					m.activeScreen = createPolicy
					m.statusMessage = "Creating new policy..."
				}
				return m, nil
			}
		}

		// Show client selector with appropriate message
		switch operation {
		case "view":
			m.statusMessage = "Select a client to view their policy"
		case "edit":
			m.statusMessage = "Select a client to edit their policy"
		case "create":
			m.statusMessage = "Select a client to create a policy"
		case "delete":
			m.statusMessage = "Select a client to delete their policy"
		case "export":
			m.statusMessage = "Select a client to export their policy"
		}
		return m, nil

	case policyLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading policy: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}

		// Route based on the stored operation context
		// Check statusMessage to determine operation
		if strings.Contains(m.statusMessage, "view") {
			m.selectedPolicy = newPolicyDetailModel(msg.policy)
			m.selectedPolicy.setSize(m.width, m.height)
			m.activeScreen = viewPolicy
			m.statusMessage = fmt.Sprintf("Viewing policy for '%s'", msg.policy.ClientName)
		} else if strings.Contains(m.statusMessage, "edit") {
			m.policyEditorModel = newPolicyEditorModel(msg.policy.ClientName, &msg.policy)
			m.activeScreen = editPolicy
			m.statusMessage = fmt.Sprintf("Editing policy for '%s'", msg.policy.ClientName)
		} else if strings.Contains(m.statusMessage, "delete") {
			m.policyDeleteModel = newPolicyDeleteModel(msg.policy.ClientName, msg.policy)
			m.activeScreen = deletePolicy
			m.statusMessage = fmt.Sprintf("Confirm deletion for '%s'", msg.policy.ClientName)
		} else if strings.Contains(m.statusMessage, "export") {
			m.selectedPolicy = newPolicyDetailModel(msg.policy)
			m.activeScreen = policyExport
			m.statusMessage = "Press 'j' for JSON or 'y' for YAML"
		}
		return m, nil
	}
	return m, nil
}

// updateCreatePolicy handles policy creation workflow
func (m *model) updateCreatePolicy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			m.policyEditorModel = nil
			return m, nil
		}

		// Handle form updates
		if m.policyEditorModel != nil && m.policyEditorModel.form != nil {
			form, cmd := m.policyEditorModel.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.policyEditorModel.form = f

				// Check if form is completed
				if f.State == huh.StateCompleted {
					// Apply template if selected
					if m.policyEditorModel.selectedTemplate != "" {
						m.policyEditorModel.applyTemplate()
					}

					// Build and save policy
					pol := m.policyEditorModel.buildPolicy()

					// Validate
					valid, warnings := validatePolicyWithFeedback(pol)
					if !valid {
						m.statusMessage = fmt.Sprintf("❌ Invalid policy: %s", warnings[0])
						return m, nil
					}

					if len(warnings) > 0 {
						m.statusMessage = fmt.Sprintf("⚠️ Warning: %s", warnings[0])
					}

					// Save policy
					m.statusMessage = "Saving policy..."
					return m, savePolicyCmd(m.config, pol)
				}
			}
			return m, cmd
		}
	case policySavedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error saving policy: %v", msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("✓ Policy created for '%s'", msg.clientName)
		}
		m.activeScreen = policyManagement
		m.policyEditorModel = nil
		return m, nil
	}
	return m, nil
}

// updateEditPolicy handles policy editing workflow
func (m *model) updateEditPolicy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			m.policyEditorModel = nil
			return m, nil
		}

		// Handle form updates similar to create
		if m.policyEditorModel != nil && m.policyEditorModel.form != nil {
			form, cmd := m.policyEditorModel.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.policyEditorModel.form = f

				if f.State == huh.StateCompleted {
					pol := m.policyEditorModel.buildPolicy()

					// Validate
					valid, warnings := validatePolicyWithFeedback(pol)
					if !valid {
						m.statusMessage = fmt.Sprintf("❌ Invalid policy: %s", warnings[0])
						return m, nil
					}

					if len(warnings) > 0 {
						m.statusMessage = fmt.Sprintf("⚠️ Warning: %s", warnings[0])
					}

					// Save policy
					m.statusMessage = "Saving changes..."
					return m, savePolicyCmd(m.config, pol)
				}
			}
			return m, cmd
		}
	case policyLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading policy: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}

		// Initialize editor with existing policy
		m.policyEditorModel = newPolicyEditorModel(msg.policy.ClientName, &msg.policy)
		m.statusMessage = fmt.Sprintf("Editing policy for '%s'", msg.policy.ClientName)
		return m, nil
	case policySavedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error saving policy: %v", msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("✓ Policy updated for '%s'", msg.clientName)
		}
		m.activeScreen = policyManagement
		m.policyEditorModel = nil
		return m, nil
	}
	return m, nil
}

// updateDeletePolicy handles policy deletion with confirmation
func (m *model) updateDeletePolicy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			m.policyDeleteModel = nil
			return m, nil
		}

		// Handle confirmation form
		if m.policyDeleteModel != nil && m.policyDeleteModel.form != nil {
			form, cmd := m.policyDeleteModel.form.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.policyDeleteModel.form = f

				if f.State == huh.StateCompleted {
					if m.policyDeleteModel.confirmed {
						// User confirmed deletion
						m.statusMessage = fmt.Sprintf("Deleting policy for '%s'...", m.policyDeleteModel.clientName)
						return m, deletePolicyCmd(m.config, m.policyDeleteModel.clientName)
					} else {
						// User cancelled
						m.statusMessage = "Deletion cancelled"
						m.activeScreen = policyManagement
						m.policyDeleteModel = nil
						return m, nil
					}
				}
			}
			return m, cmd
		}
	case policyLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading policy: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}

		// Show confirmation dialog
		m.policyDeleteModel = newPolicyDeleteModel(msg.policy.ClientName, msg.policy)
		m.statusMessage = fmt.Sprintf("Confirm deletion for '%s'", msg.policy.ClientName)
		return m, nil
	case policyDeletedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error deleting policy: %v", msg.err)
		} else {
			m.statusMessage = fmt.Sprintf("✓ Policy deleted for '%s'", msg.clientName)
		}
		m.activeScreen = policyManagement
		m.policyDeleteModel = nil
		return m, nil
	}
	return m, nil
}

// updatePolicyExport handles policy export
func (m *model) updatePolicyExport(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.activeScreen = policyManagement
			m.statusMessage = ""
			return m, nil
		case "j", "1":
			// Export as JSON
			if m.selectedPolicy != nil {
				err := exportPolicyToFile(m.selectedPolicy.policy, "json")
				if err != nil {
					m.statusMessage = fmt.Sprintf("❌ Export failed: %v", err)
				} else {
					filename := fmt.Sprintf("%s-policy-%s.json", m.selectedPolicy.policy.ClientName, time.Now().Format("20060102-150405"))
					m.statusMessage = fmt.Sprintf("✓ Exported to %s", filename)
				}
				m.activeScreen = policyManagement
				return m, nil
			}
		case "y", "2":
			// Export as YAML
			if m.selectedPolicy != nil {
				err := exportPolicyToFile(m.selectedPolicy.policy, "yaml")
				if err != nil {
					m.statusMessage = fmt.Sprintf("❌ Export failed: %v", err)
				} else {
					filename := fmt.Sprintf("%s-policy-%s.yaml", m.selectedPolicy.policy.ClientName, time.Now().Format("20060102-150405"))
					m.statusMessage = fmt.Sprintf("✓ Exported to %s", filename)
				}
				m.activeScreen = policyManagement
				return m, nil
			}
		}
	case policyLoadedMsg:
		if msg.err != nil {
			m.statusMessage = fmt.Sprintf("❌ Error loading policy: %v", msg.err)
			m.activeScreen = policyManagement
			return m, nil
		}

		// Store policy and show export options
		m.selectedPolicy = newPolicyDetailModel(msg.policy)
		m.statusMessage = "Press 'j' for JSON or 'y' for YAML"
		return m, nil
	}
	return m, nil
}
