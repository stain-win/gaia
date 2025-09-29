package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	"github.com/stain-win/gaia/apps/gaia/encrypt"
)

// initCmd is the Cobra command for `gaia init`.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Gaia secure storage",
	Long: `The init command guides you through the process of setting up Gaia's encrypted database and master passphrase.

This is a one-time operation. Once the database is initialized, this command will not run again unless the database file is deleted.
`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := gaiaDaemon.GetConfig()
		if dbFile != "" {
			cfg.DBFile = dbFile
		}

		if _, err := os.Stat(cfg.DBFile); err == nil {
			fmt.Printf("Gaia is already initialized. Database file found at '%s'.\n", cfg.DBFile)
			fmt.Println("To re-initialize, please delete the existing database file first.")
			os.Exit(1)
		}

		var passphrase string
		var confirm bool

		// Create the form.
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Welcome to Gaia!").
					Description("Let's get your secure storage set up."),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Choose a master passphrase").
					Description("This will be used to encrypt your data. Do not forget it!").
					Value(&passphrase).
					Password(true).
					Validate(func(s string) error {
						if len(s) < 8 {
							return errors.New("passphrase must be at least 8 characters")
						}
						_, err := encrypt.ValidatePassword(s)
						return err
					}),
				huh.NewInput().
					Title("Confirm your passphrase").
					Password(true).
					Validate(func(s string) error {
						if s != passphrase {
							return errors.New("passphrases do not match")
						}
						return nil
					}),
			),
			huh.NewGroup(
				huh.NewConfirm().
					Title("Ready to Go?").
					Description("This will create the encrypted database file.\nPress Enter to confirm or Esc to cancel.").
					Value(&confirm),
			),
		)

		// Run the form.
		err := form.Run()
		if err != nil {
			// This catches Ctrl+C and other potential errors.
			fmt.Println("\nInitialization cancelled.")
			os.Exit(1)
		}

		if !confirm {
			fmt.Println("Initialization not confirmed. Aborting.")
			os.Exit(0)
		}

		// Initialize the database.
		gaiaDaemon := daemon.NewDaemon(cfg)
		err = gaiaDaemon.InitializeDB(passphrase)
		if err != nil {
			fmt.Printf("\nFailed to initialize database: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nGaia encrypted database initialized successfully!")
		fmt.Printf("Your database file is located at: %s\n", cfg.DBFile)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&dbFile, "db-file", "d", "", "The path to the BoltDB file")
}
