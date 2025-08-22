// ABOUTME: Command-line interface for listing ECS task definition revisions
// ABOUTME: Handles user input and output formatting for revision listing

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

func newRevisionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "revisions",
		Short:                 "List active task definitions",
		DisableFlagsInUseLine: true,
		RunE:                  revisionsHandler,
	}

	cmd.PersistentFlags().IntP("last", "", 0, "last N revisions")

	return cmd
}

func revisionsHandler(cmd *cobra.Command, args []string) error {
	revNr, _ := cmd.Flags().GetInt("last")

	// Set up context that cancels on interrupt signal for cancellable revisions operations
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}
	result, err := ecs.Revisions(ctx, clients, cluster, service, revNr)
	if err != nil {
		return fmt.Errorf("failed to get revisions: %w", err)
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
			cmd.Printf("%s: %s:%s\n", formattedDate, repo, styledTag)
		} else {
			cmd.Printf("%s: %s\n", formattedDate, revision.DockerURI)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(newRevisionsCommand())
}
