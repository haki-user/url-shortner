package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

var latencyBuckets = []time.Duration{
	time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
}

type Recorder struct {
	mu         sync.Mutex
	startedAt  time.Time
	counters   map[string]uint64
	histograms map[string]*histogram
}

func NewRecorder() *Recorder {
	return &Recorder{
		startedAt:  time.Now().UTC(),
		counters:   make(map[string]uint64),
		histograms: make(map[string]*histogram),
	}
}

func (r *Recorder) IncCounter(name string) {
	if r == nil || name == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.counters[name]++
}

func (r *Recorder) ObserveDuration(name string, duration time.Duration) {
	if r == nil || name == "" {
		return
	}

	if duration < 0 {
		duration = 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	h := r.histograms[name]
	if h == nil {
		h = newHistogram()
		r.histograms[name] = h
	}

	h.observe(duration)
}

func (r *Recorder) RecordRedirect(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}

	r.IncCounter("redirect.requests.total")
	r.IncCounter("redirect.requests." + result)
	r.ObserveDuration("redirect.latency", duration)
}

func (r *Recorder) RecordCacheGet(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}

	r.IncCounter("cache.get.total")
	r.IncCounter("cache.get." + result)
	r.ObserveDuration("cache.get.latency", duration)
}

func (r *Recorder) RecordCachePut(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}

	r.IncCounter("cache.put.total")
	r.IncCounter("cache.put." + result)
	r.ObserveDuration("cache.put.latency", duration)
}

func (r *Recorder) RecordSourceLookup(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}

	r.IncCounter("source.lookup.total")
	r.IncCounter("source.lookup." + result)
	r.ObserveDuration("source.lookup.latency", duration)
}

func (r *Recorder) RecordAnalytics(result string, duration time.Duration) {
	if result == "" {
		result = "unknown"
	}

	r.IncCounter("analytics.record.total")
	r.IncCounter("analytics.record." + result)
	r.ObserveDuration("analytics.record.latency", duration)
}

func (r *Recorder) Snapshot(now time.Time) Snapshot {
	if r == nil {
		return Snapshot{
			Status:     "disabled",
			Counters:   map[string]uint64{},
			Histograms: map[string]HistogramSnapshot{},
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	counters := make(map[string]uint64, len(r.counters))
	for name, value := range r.counters {
		counters[name] = value
	}

	histograms := make(map[string]HistogramSnapshot, len(r.histograms))
	for name, h := range r.histograms {
		histograms[name] = h.snapshot()
	}

	return Snapshot{
		Status:        "ok",
		StartedAt:     r.startedAt,
		UptimeSeconds: int64(now.Sub(r.startedAt).Seconds()),
		Counters:      counters,
		Histograms:    histograms,
	}
}

type Snapshot struct {
	Status        string                       `json:"status"`
	StartedAt     time.Time                    `json:"startedAt,omitempty"`
	UptimeSeconds int64                        `json:"uptimeSeconds,omitempty"`
	Counters      map[string]uint64            `json:"counters"`
	Histograms    map[string]HistogramSnapshot `json:"histograms"`
}

type HistogramSnapshot struct {
	Count   uint64            `json:"count"`
	MinMS   float64           `json:"minMs"`
	MaxMS   float64           `json:"maxMs"`
	AvgMS   float64           `json:"avgMs"`
	P50MS   float64           `json:"p50Ms"`
	P95MS   float64           `json:"p95Ms"`
	P99MS   float64           `json:"p99Ms"`
	Buckets map[string]uint64 `json:"buckets"`
}

type histogram struct {
	count   uint64
	total   time.Duration
	min     time.Duration
	max     time.Duration
	buckets []uint64
}

func newHistogram() *histogram {
	return &histogram{
		buckets: make([]uint64, len(latencyBuckets)+1),
	}
}

func (h *histogram) observe(duration time.Duration) {
	if h.count == 0 || duration < h.min {
		h.min = duration
	}

	if duration > h.max {
		h.max = duration
	}

	h.count++
	h.total += duration

	for index, bucket := range latencyBuckets {
		if duration <= bucket {
			h.buckets[index]++
			return
		}
	}

	h.buckets[len(h.buckets)-1]++
}

func (h *histogram) snapshot() HistogramSnapshot {
	if h.count == 0 {
		return HistogramSnapshot{
			Buckets: bucketSnapshot(h.buckets),
		}
	}

	return HistogramSnapshot{
		Count:   h.count,
		MinMS:   milliseconds(h.min),
		MaxMS:   milliseconds(h.max),
		AvgMS:   milliseconds(h.total / time.Duration(h.count)),
		P50MS:   milliseconds(h.percentileUpperBound(50)),
		P95MS:   milliseconds(h.percentileUpperBound(95)),
		P99MS:   milliseconds(h.percentileUpperBound(99)),
		Buckets: bucketSnapshot(h.buckets),
	}
}

func (h *histogram) percentileUpperBound(percentile uint64) time.Duration {
	threshold := (h.count*percentile + 99) / 100
	if threshold == 0 {
		threshold = 1
	}

	var seen uint64
	for index, count := range h.buckets {
		seen += count
		if seen < threshold {
			continue
		}

		if index < len(latencyBuckets) {
			return latencyBuckets[index]
		}

		return h.max
	}

	return h.max
}

func bucketSnapshot(values []uint64) map[string]uint64 {
	snapshot := make(map[string]uint64, len(values))
	var cumulative uint64

	for index, value := range values {
		if index < len(latencyBuckets) {
			cumulative += value
			snapshot["le_"+latencyBuckets[index].String()] = cumulative
			continue
		}

		snapshot["gt_"+latencyBuckets[len(latencyBuckets)-1].String()] = value
	}

	return snapshot
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

type Handler struct {
	recorder *Recorder
	token    string
}

func NewHandler(recorder *Recorder, token string) Handler {
	return Handler{
		recorder: recorder,
		token:    token,
	}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.token == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"status": "not_found",
		})
		return
	}

	if !authorized(r, h.token) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"status": "unauthorized",
		})
		return
	}

	writeJSON(w, http.StatusOK, h.recorder.Snapshot(time.Now().UTC()))
}

func authorized(r *http.Request, expected string) bool {
	if tokenMatches(r.Header.Get("X-Diagnostics-Token"), expected) {
		return true
	}

	if tokenMatches(r.Header.Get("X-Admin-Token"), expected) {
		return true
	}

	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	bearerToken, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok {
		return false
	}

	return tokenMatches(strings.TrimSpace(bearerToken), expected)
}

func tokenMatches(actual string, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}

	actualHash := sha256.Sum256([]byte(actual))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(actualHash[:], expectedHash[:]) == 1
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(value)
}

var _ interface {
	RecordCacheGet(string, time.Duration)
	RecordCachePut(string, time.Duration)
	RecordSourceLookup(string, time.Duration)
} = (*Recorder)(nil)

var _ interface {
	RecordRedirect(string, time.Duration)
	RecordAnalytics(string, time.Duration)
} = (*Recorder)(nil)
