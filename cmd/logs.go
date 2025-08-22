package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"runecs.io/v1/internal/ecs"
)

var boldStyle = lipgloss.NewStyle().Bold(true)

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "logs",
		Short:                 "Show logs for the service",
		DisableFlagsInUseLine: true,
		RunE:                  logsHandler,
	}

	cmd.PersistentFlags().BoolP("follow", "f", false, "follow log output")
	return cmd
}

func logsHandler(cmd *cobra.Command, args []string) error {
	cluster, service, err := parseServiceFlag()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	profile := rootCmd.Flag("profile").Value.String()
	clients, err := ecs.NewAWSClients(ctx, profile)
	if err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %w", err)
	}

	follow, err := cmd.Flags().GetBool("follow")
	if err != nil {
		return fmt.Errorf("failed to get follow flag: %w", err)
	}

	if follow {
		return followLogs(ctx, clients, cluster, service)
	}

	return showLogs(ctx, clients, cluster, service)
}

func showLogs(ctx context.Context, clients *ecs.AWSClients, cluster, service string) error {
	fmt.Printf("Fetching logs from the last hour for service %s...\n", boldStyle.Render(cluster+"/"+service))

	oneHourAgo := time.Now().Add(-time.Hour).Unix() * 1000
	logs, err := ecs.GetServiceLogs(ctx, clients, cluster, service, &oneHourAgo)
	if err != nil {
		return fmt.Errorf("failed to get logs for service %s/%s: %w", cluster, service, err)
	}

	if len(logs) == 0 {
		fmt.Println("No logs found in the last hour")
		return nil
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp < logs[j].Timestamp
	})

	for _, log := range logs {
		timestamp := time.Unix(log.Timestamp/1000, (log.Timestamp%1000)*1000000)
		fmt.Printf("%s %s\n", timestamp.Format("2006-01-02 15:04:05"), log.Message)
	}

	fmt.Printf("\nDisplayed %d log entries\n", len(logs))
	return nil
}

func followLogs(ctx context.Context, clients *ecs.AWSClients, cluster, service string) error {
	fmt.Printf("Starting live tail for service %s...\n", boldStyle.Render(cluster+"/"+service))
	logChan, closeFunc, err := ecs.TailServiceLogs(ctx, clients, cluster, service)
	if err != nil {
		return fmt.Errorf("failed to start tailing logs: %w", err)
	}

	defer closeFunc()

	fmt.Println("Connected. Streaming logs (press Ctrl+C to stop)...")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nLog stream closed")
			return nil
		case log, ok := <-logChan:
			if !ok {
				fmt.Println("\nLog stream closed")
				return nil
			}

			timestamp := time.Unix(log.Timestamp/1000, (log.Timestamp%1000)*1000000)
			fmt.Printf("%s %s\n", timestamp.Format("2006-01-02 15:04:05"), log.Message)
		}
	}
}

func init() {
	rootCmd.AddCommand(newLogsCommand())
}
