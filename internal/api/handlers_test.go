package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"strings"
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
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
			wantStatus: http.StatusOK,
		},
		{
			name:       "upper bound valid",
			body:       `{"timestamp":"23:59","service":"api","level":"info","message":"ok"}`,
			wantStatus: http.StatusOK,
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

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", postRec.Code)
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
				if rec.Code != http.StatusOK {
					t.Errorf("expected status 200, got %d", rec.Code)
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

func TestStoreAppendPrintsInvalidEventError(t *testing.T) {
	store := logstore.NewStore()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = writer

	appendErr := store.Append(model.LogEvent{
		Timestamp: "99:99",
		Service:   "api",
		Level:     "info",
		Message:   "bad",
	})

	_ = writer.Close()
	os.Stdout = originalStdout

	if appendErr == nil {
		t.Fatal("expected append to return error")
	}

	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}

	if !strings.Contains(output.String(), "invalid log event:") {
		t.Fatalf("expected invalid log output, got %q", output.String())
	}
}
