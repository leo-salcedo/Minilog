package model

import "testing"

func TestLogValidate(t *testing.T) {
	tests := []struct {
		name    string
		log     *LogEvent
		wantErr bool
	}{
		{
			name: "valid log",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "INFO",
				Message:   "request completed",
				Attributes: map[string]string{
					"request_id": "123",
				},
			},
		},
		{
			name: "missing timestamp",
			log: &LogEvent{
				Service: "api",
				Level:   "info",
				Message: "request completed",
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp format",
			log: &LogEvent{
				Timestamp: "2026-04-24T10:00:00Z",
				Service:   "api",
				Level:     "info",
				Message:   "request completed",
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp range",
			log: &LogEvent{
				Timestamp: "24:00",
				Service:   "api",
				Level:     "info",
				Message:   "request completed",
			},
			wantErr: true,
		},
		{
			name: "missing service",
			log: &LogEvent{
				Timestamp: "10:00",
				Level:     "info",
				Message:   "request completed",
			},
			wantErr: true,
		},
		{
			name: "missing message",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "info",
			},
			wantErr: true,
		},
		{
			name: "missing level",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Message:   "request completed",
			},
			wantErr: true,
		},
		{
			name: "invalid level",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "fatal",
				Message:   "request completed",
			},
			wantErr: true,
		},
		{
			name: "empty attribute key",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "info",
				Message:   "request completed",
				Attributes: map[string]string{
					"": "123",
				},
			},
			wantErr: true,
		},
		{
			name: "empty attribute value",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "info",
				Message:   "request completed",
				Attributes: map[string]string{
					"request_id": "",
				},
			},
			wantErr: true,
		},
		{
			name: "validation is read only",
			log: &LogEvent{
				Timestamp: "10:00",
				Service:   "api",
				Level:     "INFO",
				Message:   "request completed",
			},
		},
		{
			name:    "nil log",
			log:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLevel := ""
			if tt.log != nil {
				originalLevel = tt.log.Level
			}

			err := tt.log.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}

			if tt.log.Level != originalLevel {
				t.Fatalf("expected validation to be read-only, level changed from %q to %q", originalLevel, tt.log.Level)
			}
		})
	}
}
