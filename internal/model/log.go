package model

import (
	"fmt"
	"strconv"
	"strings"
)

var validLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

type LogEvent struct {
	Timestamp  string            `json:"timestamp"`
	Service    string            `json:"service"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func (l *LogEvent) Validate() error {
	if l == nil {
		return fmt.Errorf("log is required")
	}

	if strings.TrimSpace(l.Timestamp) == "" {
		return fmt.Errorf("timestamp is required")
	}

	if !isValidTimestamp(l.Timestamp) {
		return fmt.Errorf("timestamp must be in HH:MM format between 00:00 and 23:59")
	}

	if strings.TrimSpace(l.Service) == "" {
		return fmt.Errorf("service is required")
	}

	if strings.TrimSpace(l.Message) == "" {
		return fmt.Errorf("message is required")
	}

	level := strings.ToLower(strings.TrimSpace(l.Level))
	if level == "" {
		return fmt.Errorf("level is required")
	}

	if _, ok := validLevels[level]; !ok {
		return fmt.Errorf("level must be one of debug/info/warn/error")
	}

	for key, value := range l.Attributes {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("attribute keys must not be empty")
		}

		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("attribute values must not be empty")
		}
	}

	return nil
}

func isValidTimestamp(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}

	hours, err := strconv.Atoi(value[:2])
	if err != nil {
		return false
	}

	minutes, err := strconv.Atoi(value[3:])
	if err != nil {
		return false
	}

	return hours >= 0 && hours <= 23 && minutes >= 0 && minutes <= 59
}
