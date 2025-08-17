package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs",
		Short:                 "Show logs for the service",
		DisableFlagsInUseLine: true,
		RunE:                  logsHandler,
	}

	return cmd
}

func logsHandler(cmd *cobra.Command, args []string) error {
	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}

	fmt.Printf("hello world - would show logs for %s/%s\n", cluster, service)
	return nil
}

func init() {
	rootCmd.AddCommand(newLogsCommand())
}
