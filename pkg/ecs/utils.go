package ecs

import (
	"fmt"
	"strings"
	"time"
)

func extractProcessID(taskArn string) string {
	lastSlashIndex := strings.LastIndex(taskArn, "/")
	if lastSlashIndex != -1 {
		return taskArn[lastSlashIndex+1:]
	}

	return taskArn
}

func extractLastPart(arn string) string {
	parts := strings.Split(arn, "/")

	return parts[len(parts)-1]
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
