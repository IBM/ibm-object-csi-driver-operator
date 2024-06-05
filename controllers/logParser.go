// Package controllers ...
package controllers

import (
	"regexp"
	"strings"
)

// LogEntry ...
type LogEntry struct {
	PID       string
	Timestamp string
	File      string
	Message   string
}

func getLogEntry(log string) *LogEntry {
	logPattern := regexp.MustCompile(`^([A-Z]\d{4} \d{2}:\d{2}:\d{2}\.\d{6})\s+\d+\s+([\w/.-]+:\d+)\]\s+(.*)$`)

	matches := logPattern.FindStringSubmatch(log)

	if len(matches) == 4 {
		pid := matches[0]
		timestamp := matches[1]
		file := matches[2]
		message := matches[3]

		return &LogEntry{
			PID:       pid,
			Timestamp: timestamp,
			File:      file,
			Message:   message,
		}
	}

	return nil
}

func parseLogs(nodePodLogs string) map[string]string {
	logs := strings.Split(strings.TrimSpace(nodePodLogs), "\n")

	volumesStats := map[string]string{}

	for _, log := range logs {
		logEntry := getLogEntry(log)

		if logEntry != nil {
			logMsg := logEntry.Message

			regexToGetMap := regexp.MustCompile(`map\[(.*?)\]`)
			matches := regexToGetMap.FindStringSubmatch(logMsg)

			if len(matches) == 2 {
				mapContent := matches[1]

				getVolumeID := regexp.MustCompile(`VolumeId:\S+`).FindStringSubmatch(mapContent)
				if len(getVolumeID) != 0 {
					volumeID := strings.Split(getVolumeID[0], ":")[1]

					getErrMsg := strings.ReplaceAll(mapContent, getVolumeID[0], "")
					errMsg := strings.Split(getErrMsg, ":")[1]

					volumesStats[volumeID] = errMsg
				}
			}
		}
	}

	return volumesStats
}
