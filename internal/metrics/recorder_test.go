package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecorderSnapshotIncludesCountersAndLatency(t *testing.T) {
	recorder := NewRecorder()

	recorder.RecordRedirect("success", 2*time.Millisecond)
	recorder.RecordRedirect("not_found", 8*time.Millisecond)
	recorder.RecordCacheGet("hit", 500*time.Microsecond)

	snapshot := recorder.Snapshot(recorder.startedAt.Add(3 * time.Second))

	if snapshot.Status != "ok" {
		t.Fatalf("expected status ok, got %q", snapshot.Status)
	}

	if snapshot.UptimeSeconds != 3 {
		t.Fatalf("expected uptime 3 seconds, got %d", snapshot.UptimeSeconds)
	}

	if snapshot.Counters["redirect.requests.total"] != 2 {
		t.Fatalf("expected 2 redirect requests, got %d", snapshot.Counters["redirect.requests.total"])
	}

	if snapshot.Counters["redirect.requests.success"] != 1 {
		t.Fatalf("expected 1 successful redirect, got %d", snapshot.Counters["redirect.requests.success"])
	}

	redirectLatency := snapshot.Histograms["redirect.latency"]
	if redirectLatency.Count != 2 {
		t.Fatalf("expected 2 redirect latency samples, got %d", redirectLatency.Count)
	}

	if redirectLatency.MinMS != 2 {
		t.Fatalf("expected min 2ms, got %f", redirectLatency.MinMS)
	}

	if redirectLatency.MaxMS != 8 {
		t.Fatalf("expected max 8ms, got %f", redirectLatency.MaxMS)
	}

	if redirectLatency.P95MS != 10 {
		t.Fatalf("expected p95 bucket 10ms, got %f", redirectLatency.P95MS)
	}
}

func TestHandlerRequiresToken(t *testing.T) {
	handler := NewHandler(NewRecorder(), "secret")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestHandlerReturnsSnapshotWithToken(t *testing.T) {
	recorder := NewRecorder()
	recorder.RecordCacheGet("hit", time.Millisecond)

	handler := NewHandler(recorder, "secret")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.Header.Set("Authorization", "Bearer secret")

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if snapshot.Counters["cache.get.hit"] != 1 {
		t.Fatalf("expected cache hit counter, got %d", snapshot.Counters["cache.get.hit"])
	}
}

func TestHandlerIsDisabledWithoutToken(t *testing.T) {
	handler := NewHandler(NewRecorder(), "")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}
