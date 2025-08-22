package ecs

import (
	"fmt"
	"time"
)

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
