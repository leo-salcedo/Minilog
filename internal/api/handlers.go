package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"minilog/internal/logstore"
	"minilog/internal/model"
)

type LogsHandler struct {
	store *logstore.Store
}

func NewLogsHandler(store *logstore.Store) *LogsHandler {
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
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var log model.LogEvent
	if err := decoder.Decode(&log); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
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

	writeJSON(w, http.StatusOK, map[string]int{
		"accepted": 1,
		"rejected": 0,
	})
}

func (h *LogsHandler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	logs := h.store.All()
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
