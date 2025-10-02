package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/stain-win/gaia/apps/gaia/config"
)

type screen int

const (
	mainMenu screen = iota
	dataManagement
	addRecord
	certManagement
	createCerts
	registerClient
	listRecords // New screen
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
	certMenu                list.Model
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
	namespaces              []string
	daemonStatus            string
	config                  *config.Config
	//listRecords             listRecordsModel // New model state
	inspector     *inspectorModel
	statusMessage string
}

// menuItems defines the items for the main menu.
var menuItems = []list.Item{
	menuItem{"Manage Data", "Add, view, or delete secret records"},
	menuItem{"Manage Certificates", "View and manage your certificates"},
	menuItem{"Quit", "Exit the Gaia application (q)"},
}

// dataMenuItems defines the items for the data management menu.
var dataMenuItems = []list.Item{
	menuItem{"Add New Record", "Add a new secret record to Gaia"},
	menuItem{"List All Records", "View records in the database"},
	menuItem{"Back", "Return to the main menu (b)"},
}

// certMenuItems defines the items for the certificate management menu.
var certMenuItems = []list.Item{
	menuItem{"Create New Certificates", "Generate a new set of mTLS certificates"},
	menuItem{"Register Client", "Register a new client for namespacing and mTLS"},
	menuItem{"List Existing Certificates", "View all certificates known to Gaia"},
	menuItem{"Back", "Return to the main menu (b)"},
}

// initialModel creates the starting state of the TUI.
func initialModel(config *config.Config) *model {
	mainList := list.New(menuItems, list.NewDefaultDelegate(), 0, 0)
	mainList.Title = titleStyle.
		Render("Main Menu")

	mainList.SetShowStatusBar(false)
	mainList.SetFilteringEnabled(false)

	dataList := list.New(dataMenuItems, list.NewDefaultDelegate(), 0, 0)
	dataList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Data Management")

	dataList.SetShowStatusBar(false)
	dataList.SetFilteringEnabled(false)

	certList := list.New(certMenuItems, list.NewDefaultDelegate(), 0, 0)
	certList.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF8C00")).
		Render("Certificate Management")

	certList.SetShowStatusBar(false)
	certList.SetFilteringEnabled(false)

	m := model{
		activeScreen:            mainMenu,
		mainMenu:                mainList,
		dataMenu:                dataList,
		certMenu:                certList,
		help:                    help.New(),
		addRecordFormModel:      newAddRecordFormModel(nil, nil),
		registerClientFormModel: newRegisterClientFormModel(),
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
