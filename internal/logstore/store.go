package logstore

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"minilog/internal/model"
)

var errStoreRequired = errors.New("store is required")

type QueryOptions struct {
	Level    string
	Service  string
	Contains string
	Limit    int
	HasLimit bool
}

func (opts QueryOptions) Validate() error {
	if opts.Level != "" && strings.TrimSpace(opts.Level) == "" {
		return fmt.Errorf("level must not be blank")
	}
	if opts.Service != "" && strings.TrimSpace(opts.Service) == "" {
		return fmt.Errorf("service must not be blank")
	}
	if opts.Limit < 0 {
		return fmt.Errorf("limit must be a positive integer")
	}
	if opts.HasLimit && opts.Limit == 0 {
		return fmt.Errorf("limit must be a positive integer")
	}

	return nil
}

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
	logs, _ := s.Query(QueryOptions{})
	return logs
}

func (s *Store) Query(opts QueryOptions) ([]model.LogEvent, error) {
	if s == nil {
		return nil, nil
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	level := strings.ToLower(strings.TrimSpace(opts.Level))
	service := strings.TrimSpace(opts.Service)
	contains := opts.Contains

	logs := make([]model.LogEvent, 0, len(s.logs))
	for i := range s.logs {
		log := s.logs[i]
		if level != "" && strings.ToLower(strings.TrimSpace(log.Level)) != level {
			continue
		}
		if service != "" && strings.TrimSpace(log.Service) != service {
			continue
		}
		if contains != "" && !strings.Contains(log.Message, contains) {
			continue
		}

		logs = append(logs, cloneLogEvent(log))
		if opts.HasLimit && len(logs) >= opts.Limit {
			break
		}
	}

	return logs, nil
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
