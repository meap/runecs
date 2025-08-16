package ecs

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// extractARNResource extracts the resource identifier from an AWS ARN
// using the official AWS SDK ARN parser
func extractARNResource(arnString string) (string, error) {
	parsedARN, err := arn.Parse(arnString)
	if err != nil {
		return "", fmt.Errorf("failed to parse ARN %q: %w", arnString, err)
	}

	// Extract the resource ID from the Resource field
	// For resources like "task/abc123" or "user/David", get the part after "/"
	if idx := strings.LastIndex(parsedARN.Resource, "/"); idx != -1 {
		return parsedARN.Resource[idx+1:], nil
	}

	// If no "/" in resource, return the whole resource
	return parsedARN.Resource, nil
}

// formatRunningTime formats a duration into a human-readable string.
// For durations >= 24 hours, it shows days, hours, and minutes (e.g., "2d 5h 30m").
// For durations < 24 hours, it shows hours and minutes (e.g., "5h 30m").
func formatRunningTime(duration time.Duration) string {
	totalHours := int(duration.Hours())

	if totalHours >= 24 {
		days := totalHours / 24
		hours := totalHours % 24
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else {
		hours := totalHours
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}
