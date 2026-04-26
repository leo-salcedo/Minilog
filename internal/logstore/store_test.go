package logstore

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"sync"
	"testing"

	"minilog/internal/model"
)

func TestNewStoreStartsEmpty(t *testing.T) {
	store := NewStore()

	if got := len(store.All()); got != 0 {
		t.Fatalf("expected empty store, got %d logs", got)
	}
}

func TestAppendAddsOneLog(t *testing.T) {
	store := NewStore()
	log := model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "request completed",
	}

	if err := store.Append(log); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}

	logs := store.All()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	if !reflect.DeepEqual(logs[0], log) {
		t.Fatalf("expected appended log %+v, got %+v", log, logs[0])
	}
}

func TestAppendSkipsInvalidLog(t *testing.T) {
	store := NewStore()

	err := store.Append(model.LogEvent{
		Timestamp: "99:99",
		Service:   "api",
		Level:     "info",
		Message:   "request completed",
	})
	if err == nil {
		t.Fatal("expected invalid log append to return error")
	}

	if got := len(store.All()); got != 0 {
		t.Fatalf("expected invalid log to be skipped, got %d logs", got)
	}
}

func TestAllReturnsAllLogs(t *testing.T) {
	store := NewStore()
	logs := []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
	}

	for _, log := range logs {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got := store.All()
	if len(got) != len(logs) {
		t.Fatalf("expected %d logs, got %d", len(logs), len(got))
	}

	for i := range logs {
		if !reflect.DeepEqual(got[i], logs[i]) {
			t.Fatalf("expected log at index %d to be %+v, got %+v", i, logs[i], got[i])
		}
	}
}

func TestQueryByLevel(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "INFO", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Level: "info"})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "one" {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestQueryByService(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Service: "  api  "})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 logs, got %+v", got)
	}
}

func TestQueryByContains(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "request started"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "job failed"},
		{Timestamp: "10:02", Service: "api", Level: "error", Message: "request completed"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Contains: "request"})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 logs, got %+v", got)
	}
}

func TestQueryWithCombinedFilters(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "request started"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "background task"},
		{Timestamp: "10:02", Service: "worker", Level: "info", Message: "request started"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{
		Level:    "INFO",
		Service:  "api",
		Contains: "request",
	})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "request started" {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestQueryWithLimit(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "one"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "two"},
		{Timestamp: "10:02", Service: "api", Level: "info", Message: "three"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Limit: 2, HasLimit: true})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 2 || got[0].Message != "one" || got[1].Message != "two" {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestQueryRejectsBlankLevel(t *testing.T) {
	store := NewStore()

	_, err := store.Query(QueryOptions{Level: "   "})
	if err == nil || err.Error() != "level must not be blank" {
		t.Fatalf("expected blank level error, got %v", err)
	}
}

func TestQueryRejectsBlankService(t *testing.T) {
	store := NewStore()

	_, err := store.Query(QueryOptions{Service: "   "})
	if err == nil || err.Error() != "service must not be blank" {
		t.Fatalf("expected blank service error, got %v", err)
	}
}

func TestQueryRejectsZeroLimit(t *testing.T) {
	store := NewStore()

	_, err := store.Query(QueryOptions{Limit: 0, HasLimit: true})
	if err == nil || err.Error() != "limit must be a positive integer" {
		t.Fatalf("expected zero limit error, got %v", err)
	}
}

func TestQueryRejectsNegativeLimit(t *testing.T) {
	store := NewStore()

	_, err := store.Query(QueryOptions{Limit: -1, HasLimit: true})
	if err == nil || err.Error() != "limit must be a positive integer" {
		t.Fatalf("expected negative limit error, got %v", err)
	}
}

func TestQueryLevelTrimsWhitespace(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "INFO", Message: "one"},
		{Timestamp: "10:01", Service: "worker", Level: "warn", Message: "two"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Level: "  info  "})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "one" {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestQueryContainsIsCaseSensitive(t *testing.T) {
	store := NewStore()
	for _, log := range []model.LogEvent{
		{Timestamp: "10:00", Service: "api", Level: "info", Message: "Request started"},
		{Timestamp: "10:01", Service: "api", Level: "info", Message: "request completed"},
	} {
		if err := store.Append(log); err != nil {
			t.Fatalf("expected append to succeed, got error: %v", err)
		}
	}

	got, err := store.Query(QueryOptions{Contains: "request"})
	if err != nil {
		t.Fatalf("expected query to succeed, got error: %v", err)
	}
	if len(got) != 1 || got[0].Message != "request completed" {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	store := NewStore()
	if err := store.Append(model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "original",
	}); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}

	logs := store.All()
	logs[0] = model.LogEvent{
		Timestamp: "10:01",
		Service:   "worker",
		Level:     "error",
		Message:   "modified",
	}

	stored := store.All()
	if stored[0].Message != "original" {
		t.Fatalf("expected store to remain unchanged, got %+v", stored[0])
	}
}

func TestAppendSnapshotsAttributes(t *testing.T) {
	store := NewStore()
	log := model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "original",
		Attributes: map[string]string{
			"request_id": "123",
		},
	}

	if err := store.Append(log); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}
	log.Attributes["request_id"] = "mutated"

	stored := store.All()
	if stored[0].Attributes["request_id"] != "123" {
		t.Fatalf("expected stored attributes to be isolated, got %+v", stored[0].Attributes)
	}
}

func TestAllReturnsDeepCopy(t *testing.T) {
	store := NewStore()
	if err := store.Append(model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "original",
		Attributes: map[string]string{
			"request_id": "123",
		},
	}); err != nil {
		t.Fatalf("expected append to succeed, got error: %v", err)
	}

	logs := store.All()
	logs[0].Attributes["request_id"] = "mutated"

	stored := store.All()
	if stored[0].Attributes["request_id"] != "123" {
		t.Fatalf("expected returned attributes to be isolated, got %+v", stored[0].Attributes)
	}
}

func TestConcurrentAppends(t *testing.T) {
	store := NewStore()

	const goroutines = 50
	const perGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(worker int) {
			defer wg.Done()

			for j := 0; j < perGoroutine; j++ {
				if err := store.Append(model.LogEvent{
					Timestamp: "10:00",
					Service:   "api",
					Level:     "info",
					Message:   "log-" + strconv.Itoa(worker) + "-" + strconv.Itoa(j),
				}); err != nil {
					t.Errorf("expected append to succeed, got error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	want := goroutines * perGoroutine
	if got := len(store.All()); got != want {
		t.Fatalf("expected %d logs, got %d", want, got)
	}
}

func TestConcurrentAppendAndAll(t *testing.T) {
	store := NewStore()

	const writers = 20
	const perWriter = 50
	const readers = 10

	var writeWG sync.WaitGroup
	writeWG.Add(writers)

	var readWG sync.WaitGroup
	readWG.Add(readers)

	var writerDone atomic.Bool

	for i := 0; i < writers; i++ {
		go func(worker int) {
			defer writeWG.Done()

			for j := 0; j < perWriter; j++ {
				err := store.Append(model.LogEvent{
					Timestamp: "10:00",
					Service:   "api",
					Level:     "info",
					Message:   "log-" + strconv.Itoa(worker) + "-" + strconv.Itoa(j),
					Attributes: map[string]string{
						"worker": strconv.Itoa(worker),
					},
				})
				if err != nil {
					t.Errorf("expected append to succeed, got error: %v", err)
				}
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		go func() {
			defer readWG.Done()

			for !writerDone.Load() {
				logs := store.All()
				for _, log := range logs {
					if log.Timestamp == "" || log.Service == "" || log.Level == "" || log.Message == "" {
						t.Error("encountered partially initialized log snapshot")
						return
					}

					if log.Attributes != nil {
						log.Attributes["worker"] = "mutated"
					}
				}
			}
		}()
	}

	writeWG.Wait()
	writerDone.Store(true)
	readWG.Wait()

	want := writers * perWriter
	got := store.All()
	if len(got) != want {
		t.Fatalf("expected %d logs, got %d", want, len(got))
	}

	for _, log := range got {
		if log.Attributes == nil {
			t.Fatal("expected attributes to be present")
		}
		if log.Attributes["worker"] == "mutated" {
			t.Fatal("expected store snapshots to be isolated from reader mutation")
		}
	}
}

func TestAppendOnNilStoreReturnsError(t *testing.T) {
	var store *Store

	err := store.Append(model.LogEvent{
		Timestamp: "10:00",
		Service:   "api",
		Level:     "info",
		Message:   "request completed",
	})
	if err == nil {
		t.Fatal("expected nil store append to return error")
	}
}

func TestAllOnNilStoreReturnsNil(t *testing.T) {
	var store *Store

	if logs := store.All(); logs != nil {
		t.Fatalf("expected nil store All to return nil, got %#v", logs)
	}
}
