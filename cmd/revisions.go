// ABOUTME: Command-line interface for listing ECS task definition revisions
// ABOUTME: Handles user input and output formatting for revision listing

package cmd

import (
	"log"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

func newRevisionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List active task definitions",
		DisableFlagsInUseLine: true,
		Run:                   revisionsHandler,
	}

	cmd.PersistentFlags().IntP("last", "", 0, "last N revisions")
	return cmd
}

func revisionsHandler(cmd *cobra.Command, args []string) {
	revNr, _ := cmd.Flags().GetInt("last")

	svc := initService()
	result, err := svc.Revisions(revNr)
	if err != nil {
		log.Fatalln(err)
	}

	// Create formatted table with colors
	headerFmt := color.New(color.FgBlue, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	tbl := table.New("Revision", "Created At", "Docker URI")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	// Add all revision entries to the table
	for _, revision := range result.Revisions {
		tbl.AddRow(revision.Revision, revision.CreatedAt, revision.DockerURI)
	}

	// Print the formatted table
	tbl.Print()
}

func init() {
	rootCmd.AddCommand(newRevisionsCommand())
}
