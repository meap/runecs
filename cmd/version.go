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

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number and exit",
		Run:   versionHandler,
	}
}

func versionHandler(cmd *cobra.Command, args []string) {
	fmt.Printf("%s\n", version.Version)
}

func SetVersion(v *Version) {
	version = v
}

func init() {
	rootCmd.AddCommand(newVersionCommand())
}
