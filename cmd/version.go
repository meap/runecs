package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Version struct {
	Version   string
	Commit    string
	BuildTime string
}

var version *Version

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the version number and exit",
	Run:   printVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("runecs %s\n", version.Version)
}

func SetVersion(v *Version) {
	version = v
}
