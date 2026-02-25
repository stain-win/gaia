package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/huh"
	"github.com/stain-win/gaia/apps/gaia/config"
)

// messageType identifies the severity of a status message.
type messageType int

const (
	msgInfo messageType = iota
	msgSuccess
	msgError
	msgWarning
)

type screen int

const (
	mainMenu screen = iota
	dataManagement
	addRecord
	accessManagement // Renamed from certManagement
	certManagement   // Submenu for certificates
	policyManagement // New: Submenu for policies
	createCerts
	registerClient
	listRecords
	unlockScreen
	listPolicies       // New: List all policies
	viewPolicy         // New: View specific policy
	createPolicy       // New: Create/edit policy
	deletePolicy       // New: Delete policy with confirmation
	editPolicy         // New: Edit existing policy
	selectPolicyClient // New: Select client for policy operations
	policyExport       // New: Export policy
)

// A custom item type for our list.
type menuItem struct {
	title string
	desc  string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// model holds the state for the main TUI application.
type model struct {
	activeScreen            screen
	mainMenu                list.Model
	dataMenu                list.Model
	accessMenu              list.Model // New: Access management menu
	certMenu                list.Model
	policyMenu              list.Model // New: Policy management menu
	addRecordFormModel      *addRecordFormModel
	registerClientFormModel *registerClientFormModel
	quitting                bool
	certForm                *huh.Form
	caName                  string
	serverName              string
	clientName              string
	outputPath              string
	help                    help.Model
	width                   int
	height                  int
	clients                 []string
	daemonStatus            string
	config                  *config.Config
	inspector               *inspectorModel
	unlockFormModel         *unlockFormModel
	statusMessage           string
	statusMessageType       messageType
	// Policy management state
	policies           []policyListItem
	selectedPolicy     *policyDetailModel
	policyFormModel    *policyFormModel
	policyEditorModel  *policyEditorModel
	policyDeleteModel  *policyDeleteModel
	selectedClientName string // For policy operations
}

// menuItems defines the items for the main menu.
var menuItems = []list.Item{
	menuItem{"Manage Data", "Add, view, or delete secret records"},
	menuItem{"Manage Access", "Manage certificates and authorization policies"},
	menuItem{"Quit", "Exit the Gaia application (q)"},
}

// dataMenuItems defines the items for the data management menu.
var dataMenuItems = []list.Item{
	menuItem{"Add New Record", "Add a new secret record to Gaia"},
	menuItem{"List All Records", "View records in the database"},
	menuItem{"Back", "Return to the main menu (b)"},
}

// accessMenuItems defines the items for the access management menu.
var accessMenuItems = []list.Item{
	menuItem{"Manage Certificates", "View and manage mTLS certificates"},
	menuItem{"Manage Policies", "View and manage authorization policies"},
	menuItem{"Back", "Return to the main menu (b)"},
}

// certMenuItems defines the items for the certificate management menu.
var certMenuItems = []list.Item{
	menuItem{"Create New Certificates", "Generate a new set of mTLS certificates"},
	menuItem{"Register Client", "Register a new client for namespacing and mTLS"},
	menuItem{"List Existing Certificates", "View all certificates known to Gaia"},
	menuItem{"Back", "Return to access management (b)"},
}

// policyMenuItems defines the items for the policy management menu.
var policyMenuItems = []list.Item{
	menuItem{"List All Policies", "View all authorization policies"},
	menuItem{"View Policy", "View details of a specific policy"},
	menuItem{"Create New Policy", "Create a new authorization policy"},
	menuItem{"Edit Policy", "Edit an existing policy"},
	menuItem{"Delete Policy", "Remove an authorization policy"},
	menuItem{"Export Policy", "Export a policy to JSON/YAML file"},
	menuItem{"Back", "Return to access management (b)"},
}

// initialModel creates the starting state of the TUI.
func initialModel(config *config.Config) *model {
	mainList := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	mainList.Title = titleStyle.Render("Main Menu")
	mainList.SetShowStatusBar(false)
	mainList.SetFilteringEnabled(false)

	dataList := list.New(dataMenuItems, list.NewDefaultDelegate(), 0, 0)
	dataList.Title = titleStyle.Render("Data Management")
	dataList.SetShowStatusBar(false)
	dataList.SetFilteringEnabled(false)

	accessList := list.New(accessMenuItems, list.NewDefaultDelegate(), 0, 0)
	accessList.Title = titleStyle.Render("Access Management")
	accessList.SetShowStatusBar(false)
	accessList.SetFilteringEnabled(false)

	certList := list.New(certMenuItems, list.NewDefaultDelegate(), 0, 0)
	certList.Title = titleStyle.Render("Certificate Management")
	certList.SetShowStatusBar(false)
	certList.SetFilteringEnabled(false)

	policyList := list.New(policyMenuItems, list.NewDefaultDelegate(), 0, 0)
	policyList.Title = titleStyle.Render("Policy Management")
	policyList.SetShowStatusBar(false)
	policyList.SetFilteringEnabled(false)

	m := model{
		activeScreen:            mainMenu,
		mainMenu:                mainList,
		dataMenu:                dataList,
		accessMenu:              accessList,
		certMenu:                certList,
		policyMenu:              policyList,
		help:                    help.New(),
		addRecordFormModel:      newAddRecordFormModel(nil),
		registerClientFormModel: newRegisterClientFormModel(),
		unlockFormModel:         newUnlockFormModel(),
		daemonStatus:            "",
		config:                  config,
		inspector:               newInspectorModel(config),
	}

	m.certForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("caName").Title("CA Common Name").Value(&m.caName),
			huh.NewInput().Key("serverName").Title("Server Common Name").Value(&m.serverName),
			huh.NewInput().Key("clientName").Title("Client Common Name").Value(&m.clientName),
			huh.NewInput().Key("outputPath").Title("Output Directory").Placeholder("./certs").Value(&m.outputPath),
		),
	)

	return &m
}

// setInfo sets the status message with info styling.
func (m *model) setInfo(msg string) {
	m.statusMessage = msg
	m.statusMessageType = msgInfo
}

// setSuccess sets the status message with success styling.
func (m *model) setSuccess(msg string) {
	m.statusMessage = msg
	m.statusMessageType = msgSuccess
}

// setError sets the status message with error styling.
func (m *model) setError(msg string) {
	m.statusMessage = msg
	m.statusMessageType = msgError
}

// setWarning sets the status message with warning styling.
func (m *model) setWarning(msg string) {
	m.statusMessage = msg
	m.statusMessageType = msgWarning
}

// clearStatus clears the status message.
func (m *model) clearStatus() {
	m.statusMessage = ""
	m.statusMessageType = msgInfo
}
