package ecs

import "strings"

func extractProcessID(taskArn string) string {
	lastSlashIndex := strings.LastIndex(taskArn, "/")
	if lastSlashIndex != -1 {
		return taskArn[lastSlashIndex+1:]
	}

	return taskArn
}
