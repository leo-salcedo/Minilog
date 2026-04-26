package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"minilog/internal/logstore"
	"minilog/internal/model"
)

const maxLogRequestBodyBytes int64 = 1 << 20

type LogsHandler struct {
	store logStore
}

type logStore interface {
	Append(model.LogEvent) error
	All() []model.LogEvent
	Query(logstore.QueryOptions) ([]model.LogEvent, error)
}

func NewLogsHandler(store logStore) *LogsHandler {
	return &LogsHandler{store: store}
}

func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "store unavailable",
		})
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePostLogs(w, r)
	case http.MethodGet:
		h.handleGetLogs(w, r)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func (h *LogsHandler) handlePostLogs(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogRequestBodyBytes)
	defer r.Body.Close()

	raw, err := decodeSinglePayload(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": "request body too large",
			})
			return
		}

		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	switch trimmed[0] {
	case '{':
		h.handleSingleLogPost(w, trimmed)
	case '[':
		h.handleBatchLogPost(w, trimmed)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
	}
}

func (h *LogsHandler) handleSingleLogPost(w http.ResponseWriter, raw []byte) {
	log, err := decodeLogEvent(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	if err := h.store.Append(log); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int{
		"accepted": 1,
		"rejected": 0,
	})
}

func (h *LogsHandler) handleBatchLogPost(w http.ResponseWriter, raw []byte) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	type batchError struct {
		Index  int    `json:"index"`
		Reason string `json:"reason"`
	}

	response := struct {
		Accepted int          `json:"accepted"`
		Rejected int          `json:"rejected"`
		Errors   []batchError `json:"errors"`
		Error    string       `json:"error,omitempty"`
	}{
		Errors: make([]batchError, 0),
	}

	if len(items) == 0 {
		response.Error = "batch must not be empty"
		writeJSON(w, http.StatusBadRequest, response)
		return
	}

	for i, item := range items {
		log, err := decodeLogEvent(item)
		if err != nil {
			response.Rejected++
			response.Errors = append(response.Errors, batchError{
				Index:  i,
				Reason: "invalid JSON",
			})
			continue
		}

		if err := h.store.Append(log); err != nil {
			response.Rejected++
			response.Errors = append(response.Errors, batchError{
				Index:  i,
				Reason: err.Error(),
			})
			continue
		}

		response.Accepted++
	}

	status := http.StatusBadRequest
	if response.Accepted > 0 {
		status = http.StatusCreated
	}

	writeJSON(w, status, response)
}

func decodeSinglePayload(body io.Reader) ([]byte, error) {
	decoder := json.NewDecoder(body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("extra JSON values")
	}

	return raw, nil
}

func decodeLogEvent(raw []byte) (model.LogEvent, error) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return model.LogEvent{}, errors.New("invalid JSON")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var log model.LogEvent
	if err := decoder.Decode(&log); err != nil {
		return model.LogEvent{}, err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return model.LogEvent{}, errors.New("extra JSON values")
	}

	return log, nil
}

func (h *LogsHandler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	opts, err := parseQueryOptions(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	logs, err := h.store.Query(opts)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}
	if logs == nil {
		logs = []model.LogEvent{}
	}
	writeJSON(w, http.StatusOK, struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}{
		Count: len(logs),
		Logs:  logs,
	})
}

func parseQueryOptions(r *http.Request) (logstore.QueryOptions, error) {
	query := r.URL.Query()

	opts := logstore.QueryOptions{
		Level:    query.Get("level"),
		Service:  query.Get("service"),
		Contains: query.Get("contains"),
	}

	if rawLimit := query.Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(strings.TrimSpace(rawLimit))
		if err != nil {
			return logstore.QueryOptions{}, errors.New("limit must be a positive integer")
		}
		opts.Limit = limit
		opts.HasLimit = true
	}

	return opts, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
