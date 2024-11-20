package ecs

import "strings"

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
