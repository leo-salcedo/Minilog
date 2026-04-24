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
