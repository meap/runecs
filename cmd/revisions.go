// ABOUTME: Command-line interface for listing ECS task definition revisions
// ABOUTME: Handles user input and output formatting for revision listing

package cmd

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/pkg/ecs"
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

	cluster, service := parseServiceFlag()
	result, err := ecs.Revisions(cluster, service, revNr)
	if err != nil {
		log.Fatalln(err)
	}

	// Sort revisions by revision number in descending order
	sort.Slice(result.Revisions, func(i, j int) bool {
		return result.Revisions[i].Revision > result.Revisions[j].Revision
	})

	// Create lipgloss style for date formatting
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	boldStyle := lipgloss.NewStyle().Bold(true)

	// Print revisions line by line
	for _, revision := range result.Revisions {
		// Use proper Go time formatting for date and time without seconds
		formattedDate := dateStyle.Render(revision.CreatedAt.Local().Format(time.DateTime)[:16])

		// Split DockerURI to extract and style the tag
		dockerParts := strings.Split(revision.DockerURI, ":")
		if len(dockerParts) >= 2 {
			repo := strings.Join(dockerParts[:len(dockerParts)-1], ":")
			tag := dockerParts[len(dockerParts)-1]
			styledTag := boldStyle.Render(tag)
			fmt.Printf("%s: %s:%s\n", formattedDate, repo, styledTag)
		} else {
			fmt.Printf("%s: %s\n", formattedDate, revision.DockerURI)
		}
	}
}

func init() {
	rootCmd.AddCommand(newRevisionsCommand())
}
