package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information - set by linker during build
var (
	// version is the semantic version
	version = "dev"
	// gitCommit is the git commit hash
	gitCommit = "unknown"
	// buildDate is the build timestamp
	buildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Gaia",
	Long:  `Display the version of Gaia along with build information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Gaia %s\n", version)
		fmt.Printf("  Commit:  %s\n", gitCommit)
		fmt.Printf("  Built:   %s\n", buildDate)
		fmt.Printf("  Go:      %s\n", runtime.Version())
		fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Println()
		fmt.Println("Run 'gaia update' to check for updates")
	},
}

// updateCmd is an alias for checking updates
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for Gaia updates",
	Long: `Check if a newer version of Gaia is available and show update instructions.

This command queries the GitHub releases API to compare your installed 
version with the latest release.`,
	RunE: runVersionCheck,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
