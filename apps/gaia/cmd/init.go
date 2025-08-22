package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
)

// These groups are now part of the `commands` package.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF8C00")). // Orange
			PaddingLeft(1)
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")). // Deep Sky Blue
			PaddingLeft(1)
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")). // Light Gray
			MarginTop(1)                       // Add one line of top padding
	appStyle = lipgloss.NewStyle().Margin(1, 2)
)

var (
	stepGreeting = huh.NewGroup(
		huh.NewNote().
			Title("Gaia Initialization").
			Description("Welcome to _Gaia_.\n\nBegin with setup").
			Next(true).
			NextLabel("Next"),
	)
	stepMasterPassphrase = huh.NewGroup(
		huh.NewInput().
			Key("passphrase").
			Title(titleStyle.Render("Enter master passphrase")).
			Prompt(promptStyle.Render(">")).
			EchoMode(huh.EchoMode(textinput.EchoPassword)).
			Validate(_validatePassphrase),
	)
	stepConfirmation = huh.NewGroup(
		huh.NewConfirm().
			Title("Do you want to initialize storage with master password").
			Key("confirmation").
			Affirmative("Yes!").
			Negative("No."),
	)
)

func _validatePassphrase(passphrase string) error {
	if len(passphrase) < 8 {
		return errors.New("passphrase must be at least 8 characters")
	} else {
		_, err := encrypt.ValidatePassword(passphrase)
		return err
	}
}

// A new Bubble Tea model to handle the interactive passphrase input.
type initModel struct {
	form         *huh.Form
	passphrase   string
	confirmation bool
	width        int
	height       int
	completed    bool
}

func newInitModel() *initModel {
	accessible, _ := strconv.ParseBool(os.Getenv("ACCESSIBLE"))
	form :=
		huh.NewForm(
			stepGreeting,
			stepMasterPassphrase,
			stepConfirmation,
		).WithAccessible(accessible)

	return &initModel{
		form:         form,
		passphrase:   "",
		confirmation: false,
		completed:    false,
	}
}

func (m *initModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var updatedForm tea.Model

	updatedForm, cmd = m.form.Update(msg)
	m.form = updatedForm.(*huh.Form)

	if m.form.State == huh.StateCompleted {
		m.passphrase = m.form.GetString("passphrase")
		m.confirmation = m.form.GetBool("confirmation")
		m.completed = true
		return m, tea.Quit
	}

	// Capture Ctrl+C for a clean exit.
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update the model with the new window size.
		m.height = msg.Height
		m.width = msg.Width
	}

	return m, cmd
}

func (m *initModel) View() string {
	// Render the form and append the styled help message below it.
	return appStyle.Render(
		lipgloss.Place(
			m.width,         // Width of the terminal
			m.height,        // Height of the terminal
			lipgloss.Center, // Horizontal alignment
			lipgloss.Center, // Vertical alignment
			lipgloss.JoinVertical(lipgloss.Left,
				m.form.View(),
				helpStyle.Render("Press Ctrl+C to quit"),
			),
		),
	)
}

// initCmd is the Cobra command for `gaia init`.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Gaia secure storage",
	Long:  `The init command guides you through the process of setting up Gaia's encrypted database and master passphrase.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the database file already exists.
		cfg := gaiaDaemon.GetConfig()
		err := config.WriteConfigToFile(cfg)
		if err != nil {
			fmt.Printf("failed to initialize configuration: %s", err)
			os.Exit(1)
		}
		if _, err := os.Stat(cfg.DBFile); err == nil {
			fmt.Printf("Gaia is already initialized. Database file found at '%s'.\n", cfg.DBFile)
			fmt.Println("To reinitialize, please delete the file first.")
			os.Exit(1)
		}

		fmt.Println("Initializing Gaia secure storage...")
		p := tea.NewProgram(newInitModel(), tea.WithAltScreen())
		finalModel, err := p.Run()

		if err != nil {
			fmt.Println("Initialization cancelled or failed:", err)
			return
		}

		if completedModel, ok := finalModel.(*initModel); ok {
			if !completedModel.completed {
				fmt.Println("Initialization cancelled.")
				return
			}

			if !completedModel.confirmation {
				fmt.Println("Database initialization cancelled by user.")
				return
			}

			passphrase := strings.TrimSpace(completedModel.passphrase)
			err = gaiaDaemon.InitializeDB(passphrase)
			if err != nil {
				fmt.Println("Failed to initialize database:", err)
				return
			}
			fmt.Println("Gaia encrypted database initialized.")
			fmt.Printf("Master passphrase set: '%s'\n", passphrase)
		}
	},
}
