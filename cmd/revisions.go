// ABOUTME: Command-line interface for listing ECS task definition revisions
// ABOUTME: Handles user input and output formatting for revision listing

package cmd

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
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

	// Sort revisions by revision number in descending order
	sort.Slice(result.Revisions, func(i, j int) bool {
		return result.Revisions[i].Revision > result.Revisions[j].Revision
	})

	// Create lipgloss style for date formatting
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	
	// Print bullet list with created date and docker URI
	for _, revision := range result.Revisions {
		// Use proper Go time formatting for date and time without seconds
		formattedDate := dateStyle.Render(revision.CreatedAt.Local().Format(time.DateTime)[:16])
		fmt.Printf("• %s: %s\n", formattedDate, revision.DockerURI)
	}
}

func init() {
	rootCmd.AddCommand(newRevisionsCommand())
}
