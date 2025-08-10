package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stain-win/gaia/apps/gaia/config"
	"github.com/stain-win/gaia/apps/gaia/daemon"
	"github.com/stain-win/gaia/apps/gaia/gaialog"
	"github.com/stain-win/gaia/apps/gaia/tui"
)

// gaiaDaemon is the single, global daemon instance.
var (
	cfgFile    string
	gaiaDaemon *daemon.Daemon
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gaia",
	Short: "Gaia is a secure runtime context daemon for web applications.",
	Long: `Gaia is a daemon that securely stores and provides runtime context and
credentials to web applications running on the same server.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Initialize the logger
		gaialog.Init(gaialog.LevelInfo, "gaia_audit.log", true)
		gaiaDaemon = daemon.NewDaemon(cfg)

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior: launch the interactive TUI if no command is given.
		if len(args) == 0 {
			cfg := gaiaDaemon.GetConfig()
			err := tui.Run(cfg)
			if err != nil {
				if strings.Contains(err.Error(), "open /dev/tty") {
					fmt.Println("Error: Could not open a new TTY. Please run Gaia in a real terminal (not in an IDE or redirected environment).\nDetails:", err)
				} else {
					fmt.Println("TUI exited with error:", err)
				}
			}
		} else {
			err := cmd.Help()
			if err != nil {
				return
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(certsCmd)
	rootCmd.AddCommand(clientsCmd)
	rootCmd.AddCommand(secretsCmd)
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(unlockCmd)
}
