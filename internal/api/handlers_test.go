package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"minilog/internal/logstore"
	"minilog/internal/model"
)

func TestPostLogsAcceptsValidLog(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 1 || response.Rejected != 0 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if got := len(store.All()); got != 1 {
		t.Fatalf("expected 1 stored log, got %d", got)
	}
}

func TestPostLogsAcceptsValidLogWithWhitespace(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(" \n\t {\"timestamp\":\"10:00\",\"service\":\"api\",\"level\":\"info\",\"message\":\"ok\"}\n\t "))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
}

func TestPostLogsAcceptsValidBatch(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		{"timestamp":"10:00","service":"api","level":"info","message":"one"},
		{"timestamp":"10:01","service":"worker","level":"warn","message":"two"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 2 || response.Rejected != 0 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", response.Errors)
	}

	if got := len(store.All()); got != 2 {
		t.Fatalf("expected 2 stored logs, got %d", got)
	}
}

func TestPostLogsBatchPartialFailure(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		{"timestamp":"10:00","service":"api","level":"info","message":"one"},
		{"timestamp":"10:01","level":"warn","message":"missing service"},
		{"timestamp":"10:02","service":"worker","level":"error","message":"two"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 2 || response.Rejected != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 1 {
		t.Fatalf("expected 1 error, got %+v", response.Errors)
	}

	if response.Errors[0].Index != 1 || response.Errors[0].Reason != "service is required" {
		t.Fatalf("unexpected error entry: %+v", response.Errors[0])
	}

	if got := len(store.All()); got != 2 {
		t.Fatalf("expected 2 stored logs, got %d", got)
	}
}

func TestPostLogsBatchFullyInvalid(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		{"timestamp":"24:00","service":"api","level":"info","message":"bad timestamp"},
		{"timestamp":"10:01","service":"worker","level":"fatal","message":"bad level"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 0 || response.Rejected != 2 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %+v", response.Errors)
	}

	if got := len(store.All()); got != 0 {
		t.Fatalf("expected 0 stored logs, got %d", got)
	}
}

func TestPostLogsRejectsEmptyBatch(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 0 || response.Rejected != 0 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", response.Errors)
	}

	if response.Error != "batch must not be empty" {
		t.Fatalf("expected empty batch error, got %+v", response)
	}
}

func TestPostLogsBatchRejectsNullItem(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		null,
		{"timestamp":"10:00","service":"api","level":"info","message":"ok"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 1 || response.Rejected != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 1 || response.Errors[0].Index != 0 || response.Errors[0].Reason != "invalid JSON" {
		t.Fatalf("unexpected errors: %+v", response.Errors)
	}
}

func TestPostLogsBatchRejectsScalarItem(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		123,
		{"timestamp":"10:00","service":"api","level":"info","message":"ok"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 1 || response.Rejected != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 1 || response.Errors[0].Index != 0 || response.Errors[0].Reason != "invalid JSON" {
		t.Fatalf("unexpected errors: %+v", response.Errors)
	}
}

func TestPostLogsBatchRejectsNestedArrayItem(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		[{"timestamp":"10:00","service":"api","level":"info","message":"nested"}],
		{"timestamp":"10:01","service":"api","level":"info","message":"ok"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 1 || response.Rejected != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 1 || response.Errors[0].Index != 0 || response.Errors[0].Reason != "invalid JSON" {
		t.Fatalf("unexpected errors: %+v", response.Errors)
	}
}

func TestPostLogsBatchRejectsUnknownFieldItem(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`[
		{"timestamp":"10:00","service":"api","level":"info","message":"ok","extra":"nope"},
		{"timestamp":"10:01","service":"api","level":"info","message":"good"}
	]`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var response struct {
		Accepted int `json:"accepted"`
		Rejected int `json:"rejected"`
		Errors   []struct {
			Index  int    `json:"index"`
			Reason string `json:"reason"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Accepted != 1 || response.Rejected != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}

	if len(response.Errors) != 1 || response.Errors[0].Index != 0 || response.Errors[0].Reason != "invalid JSON" {
		t.Fatalf("unexpected errors: %+v", response.Errors)
	}
}

func TestPostLogsRejectsInvalidJSON(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid JSON" {
		t.Fatalf("expected invalid JSON error, got %+v", response)
	}
}

func TestPostLogsRejectsEmptyBody(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPostLogsRejectsTrailingJSON(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok"}{"timestamp":"10:01","service":"api","level":"info","message":"extra"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	if got := len(store.All()); got != 0 {
		t.Fatalf("expected 0 stored logs, got %d", got)
	}
}

func TestPostLogsRejectsUnknownField(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok","extra":"nope"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPostLogsRejectsBadFieldTypes(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":123,"level":"info","message":"ok"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPostLogsRejectsMissingService(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","level":"info","message":"ok"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "service is required" {
		t.Fatalf("expected service validation error, got %+v", response)
	}
}

func TestPostLogsRejectsInvalidAttributes(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok","attributes":{"":"x"}}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestPostLogsTimestampBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "lower bound valid",
			body:       `{"timestamp":"00:00","service":"api","level":"info","message":"ok"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "upper bound valid",
			body:       `{"timestamp":"23:59","service":"api","level":"info","message":"ok"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "hour out of range",
			body:       `{"timestamp":"24:00","service":"api","level":"info","message":"ok"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed timestamp",
			body:       `{"timestamp":"09:0","service":"api","level":"info","message":"ok"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := logstore.NewStore()
			handler := NewLogsHandler(store)

			req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestPostAndGetPreserveLevelCasing(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	postReq := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"INFO","message":"ok"}`))
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", postRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/logs", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	var response struct {
		Logs []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Logs) != 1 || response.Logs[0].Level != "INFO" {
		t.Fatalf("expected level casing to be preserved, got %+v", response.Logs)
	}
}

func TestGetLogsReturnsStoredLogs(t *testing.T) {
	store := logstore.NewStore()
	if err := store.Append(model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "one",
	}); err != nil {
		t.Fatalf("failed to seed store: %v", err)
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 1 {
		t.Fatalf("expected count 1, got %d", response.Count)
	}

	if len(response.Logs) != 1 || response.Logs[0].Message != "one" {
		t.Fatalf("unexpected logs response: %+v", response.Logs)
	}
}

func TestGetLogsQueryByLevel(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "INFO", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?level=info", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 1 || len(response.Logs) != 1 || response.Logs[0].Message != "one" {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsQueryByService(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?service=%20api%20", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 || len(response.Logs) != 2 {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsQueryByContains(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "request started"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "job failed"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "request completed"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?contains=request", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 || len(response.Logs) != 2 {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsQueryWithCombinedFilters(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "request started"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "background task"},
		{Timestamp: "10:02", Service: "worker", Level: "info", Message: "request started"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?level=INFO&service=api&contains=request", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 1 || len(response.Logs) != 1 || response.Logs[0].Message != "request started" {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsQueryWithLimit(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "info", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=2", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 || len(response.Logs) != 2 || response.Logs[0].Message != "one" || response.Logs[1].Message != "two" {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsInvalidLimitReturns400(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "limit must be a positive integer" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestGetLogsZeroLimitReturns400(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=0", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "limit must be a positive integer" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestGetLogsNegativeLimitReturns400(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "limit must be a positive integer" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestGetLogsTrimmedLimitAccepted(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "info", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?limit=%202%20", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 || len(response.Logs) != 2 {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsBlankLevelReturns400(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?level=%20%20", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "level must not be blank" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestGetLogsBlankServiceReturns400(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs?service=%20%20", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "service must not be blank" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestGetLogsNoFiltersReturnsAllLogs(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 2 || len(response.Logs) != 2 {
		t.Fatalf("unexpected logs response: %+v", response)
	}
}

func TestGetLogsReturnsStoredLogsInAppendOrder(t *testing.T) {
	store := logstore.NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "first"},
		{Timestamp: "10:01", Service: "api", Level: "warn", Message: "second"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "third"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("failed to seed store: %v", err)
		}
	}

	handler := NewLogsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var response struct {
		Logs []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	want := []string{"first", "second", "third"}
	if len(response.Logs) != len(want) {
		t.Fatalf("expected %d logs, got %d", len(want), len(response.Logs))
	}

	for i, message := range want {
		if response.Logs[i].Message != message {
			t.Fatalf("expected message %q at index %d, got %+v", message, i, response.Logs)
		}
	}
}

func TestGetLogsReturnsEmptyArray(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Count int              `json:"count"`
		Logs  []model.LogEvent `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Count != 0 {
		t.Fatalf("expected count 0, got %d", response.Count)
	}

	if response.Logs == nil {
		t.Fatal("expected logs to be an empty array, got nil")
	}

	if len(response.Logs) != 0 {
		t.Fatalf("expected no logs, got %+v", response.Logs)
	}
}

func TestWrongMethodReturns405(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}

	if allow := rec.Header().Get("Allow"); allow != "GET, POST" {
		t.Fatalf("expected Allow header %q, got %q", "GET, POST", allow)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "method not allowed" {
		t.Fatalf("expected method not allowed error, got %+v", response)
	}
}

func TestNilStoreReturns500(t *testing.T) {
	handler := NewLogsHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestNilStorePostReturns500(t *testing.T) {
	handler := NewLogsHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestJSONResponsesSetContentType(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
	}{
		{
			name: "success",
			req:  httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"info","message":"ok"}`)),
		},
		{
			name: "error",
			req:  httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := logstore.NewStore()
			handler := NewLogsHandler(store)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, tt.req)

			if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("expected application/json content type, got %q", contentType)
			}
		})
	}
}

func TestConcurrentPostAndGet(t *testing.T) {
	store := logstore.NewStore()
	handler := NewLogsHandler(store)

	const writers = 20
	const perWriter = 25
	const readers = 10

	var writeWG sync.WaitGroup
	writeWG.Add(writers)

	var readWG sync.WaitGroup
	readWG.Add(readers)

	for i := 0; i < writers; i++ {
		go func(worker int) {
			defer writeWG.Done()
			for j := 0; j < perWriter; j++ {
				req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(`{"timestamp":"10:00","service":"api","level":"INFO","message":"ok","attributes":{"worker":"`+strings.TrimSpace(string(rune('0'+(worker%10))))+`"}}`))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusCreated {
					t.Errorf("expected status 201, got %d", rec.Code)
				}
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		go func() {
			defer readWG.Done()
			for j := 0; j < perWriter; j++ {
				req := httptest.NewRequest(http.MethodGet, "/logs", nil)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rec.Code)
					return
				}

				var response struct {
					Count int              `json:"count"`
					Logs  []model.LogEvent `json:"logs"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
					t.Errorf("failed to decode response: %v", err)
					return
				}
				if response.Count != len(response.Logs) {
					t.Errorf("expected count to match logs length, got count=%d len=%d", response.Count, len(response.Logs))
					return
				}
			}
		}()
	}

	writeWG.Wait()
	readWG.Wait()

	want := writers * perWriter
	if got := len(store.All()); got != want {
		t.Fatalf("expected %d logs, got %d", want, got)
	}
}
