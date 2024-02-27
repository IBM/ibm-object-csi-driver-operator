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

	for ind, log := range logs {
		logEntry := getLogEntry(log)
		if logEntry != nil {
			logMsg := logEntry.Message

			if strings.HasPrefix(logMsg, "NodeGetVolumeStats: Request:") {
				logMsg = strings.TrimSpace(strings.Trim(logMsg, "NodeGetVolumeStats Request:{}"))
				slice := strings.Fields(logMsg)
				data := map[string]string{}
				for _, val := range slice {
					kv := strings.Split(val, ":")
					if len(kv) == 2 {
						data[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}

				reqLog := logs[ind+2]
				logEntry := getLogEntry(reqLog)
				volumesStats[data["Id"]] = logEntry.Message
			}
		}
	}

	return volumesStats
}
