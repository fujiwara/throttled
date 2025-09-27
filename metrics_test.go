package throttled_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fujiwara/throttled"
)

func TestMetricsEndpoint(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Make some requests to generate metrics
	w := &mockResponseWriter{header: make(http.Header)}
	req, _ := http.NewRequest("GET", "/allow?key=test&rate=10&burst=10", nil)
	s.Handler.ServeHTTP(w, req)

	req, _ = http.NewRequest("GET", "/wait?key=test2&rate=1&burst=1", nil)
	s.Handler.ServeHTTP(w, req)

	// Test metrics endpoint
	req, _ = http.NewRequest("GET", "/metrics", nil)
	w = &mockResponseWriter{header: make(http.Header), body: &strings.Builder{}}
	s.Handler.ServeHTTP(w, req)

	if w.statusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.statusCode)
	}

	body := w.body.(*strings.Builder).String()
	metricsToCheck := []string{
		"throttled_requests_total",
		"throttled_request_duration_seconds",
		"throttled_rate_limit_created_total",
		"throttled_cache_size",
		"throttled_limiters_active",
	}

	for _, metric := range metricsToCheck {
		if !strings.Contains(body, metric) {
			t.Errorf("Metrics output missing %s", metric)
		}
	}
}

func TestMetricsRateLimitTracking(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Create limiter with burst of 1
	w := &mockResponseWriter{header: make(http.Header)}
	req, _ := http.NewRequest("GET", "/allow?key=metrics_test&rate=1&burst=1", nil)
	s.Handler.ServeHTTP(w, req)

	// Second request should hit rate limit
	req, _ = http.NewRequest("GET", "/allow?key=metrics_test&rate=1&burst=1", nil)
	s.Handler.ServeHTTP(w, req)

	// Check metrics
	req, _ = http.NewRequest("GET", "/metrics", nil)
	w = &mockResponseWriter{header: make(http.Header), body: &strings.Builder{}}
	s.Handler.ServeHTTP(w, req)

	body := w.body.(*strings.Builder).String()
	if !strings.Contains(body, "throttled_rate_limit_hits_total") {
		t.Error("Rate limit hits metric not found")
	}
}

type mockResponseWriter struct {
	header     http.Header
	body       io.Writer
	statusCode int
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	if m.body != nil {
		return m.body.Write(b)
	}
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}
