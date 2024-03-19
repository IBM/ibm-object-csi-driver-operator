package controllers

import (
	"regexp"
	"strings"
)

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
				pairs := strings.Fields(mapContent)
				volumeID := ""
				errMsg := ""

				for _, kv := range pairs {
					if strings.Contains(kv, ":") {
						data := strings.Split(kv, ":")
						if data[0] == "Error" {
							errMsg = data[1]
						} else {
							volumeID = data[1]
						}
					} else {
						errMsg += " " + kv
					}
				}

				volumesStats[volumeID] = errMsg
			}
		}
	}

	return volumesStats
}
