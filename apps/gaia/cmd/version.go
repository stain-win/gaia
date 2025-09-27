package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is a package-level variable that will be set by the linker during the build.
// It must be a 'var', not a 'const', so the linker can modify it.
// We initialize it with a default value for local development builds.
var version = "dev"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Gaia",
	Long:  `All software has versions. This is Gaia's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Gaia version %s\n", version)
	},
}
