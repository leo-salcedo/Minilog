package logstore

import (
	"errors"
	"sync"

	"minilog/internal/model"
)

var errStoreRequired = errors.New("store is required")

type Store struct {
	mu   sync.RWMutex
	logs []model.LogEvent
}

func NewStore() *Store {
	return &Store{
		logs: make([]model.LogEvent, 0),
	}
}

func (s *Store) Append(log model.LogEvent) error {
	if s == nil {
		return errStoreRequired
	}

	if err := log.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = append(s.logs, cloneLogEvent(log))
	return nil
}

func (s *Store) All() []model.LogEvent {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]model.LogEvent, len(s.logs))
	for i := range s.logs {
		logs[i] = cloneLogEvent(s.logs[i])
	}

	return logs
}

func cloneLogEvent(log model.LogEvent) model.LogEvent {
	cloned := log
	if log.Attributes == nil {
		return cloned
	}

	cloned.Attributes = make(map[string]string, len(log.Attributes))
	for key, value := range log.Attributes {
		cloned.Attributes[key] = value
	}

	return cloned
}
