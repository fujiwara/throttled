package throttled_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/fujiwara/throttled"
)

func TestRetryAfterHeader(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Create limiter with burst=1, rate=1 req/sec
	req1, _ := http.NewRequest("GET", "/allow?key=test_retry&rate=1&burst=1", nil)
	w1 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w1.Code)
	}

	// Second request should use the burst token and succeed
	req2, _ := http.NewRequest("GET", "/allow?key=test_retry&rate=1&burst=1", nil)
	w2 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	// Third request should hit rate limit and include Retry-After header
	req3, _ := http.NewRequest("GET", "/allow?key=test_retry&rate=1&burst=1", nil)
	w3 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w3.Code)
	}

	retryAfter := w3.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header not found")
	}

	retrySeconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Errorf("Invalid Retry-After value: %s", retryAfter)
	}

	if retrySeconds <= 0 || retrySeconds > 2 {
		t.Errorf("Expected Retry-After to be 1-2 seconds, got %d", retrySeconds)
	}
}

func TestRetryAfterWithSlowRate(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Create limiter with burst=1, rate=0.1 req/sec (10 second interval)
	req1, _ := http.NewRequest("GET", "/allow?key=test_slow&rate=0.1&burst=1", nil)
	w1 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w1.Code)
	}

	// Second request should use burst token
	req2, _ := http.NewRequest("GET", "/allow?key=test_slow&rate=0.1&burst=1", nil)
	w2 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	// Third request should hit rate limit
	req3, _ := http.NewRequest("GET", "/allow?key=test_slow&rate=0.1&burst=1", nil)
	w3 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w3.Code)
	}

	retryAfter := w3.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header not found")
	}

	retrySeconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Errorf("Invalid Retry-After value: %s", retryAfter)
	}

	// Should be around 10 seconds
	if retrySeconds < 9 || retrySeconds > 11 {
		t.Errorf("Expected Retry-After to be around 10 seconds, got %d", retrySeconds)
	}
}

func TestRetryAfterNotSetOnSuccess(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Request that should succeed
	req, _ := http.NewRequest("GET", "/allow?key=test_success&rate=10&burst=10", nil)
	w := httptest.NewRecorder()
	s.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusTooManyRequests {
		t.Errorf("Expected non-429 status, got %d", w.Code)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter != "" {
		t.Errorf("Retry-After header should not be set on successful request, got: %s", retryAfter)
	}
}

func TestRetryAfterRespectsTiming(t *testing.T) {
	s, err := throttled.NewServer(1024)
	if err != nil {
		t.Fatal(err)
	}

	// Create limiter with burst=1, rate=2 req/sec (0.5 second interval)
	req1, _ := http.NewRequest("GET", "/allow?key=test_timing&rate=2&burst=1", nil)
	w1 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w1, req1)

	// Use up the burst token
	req2, _ := http.NewRequest("GET", "/allow?key=test_timing&rate=2&burst=1", nil)
	w2 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w2, req2)

	// Get rate limit response with Retry-After
	req3, _ := http.NewRequest("GET", "/allow?key=test_timing&rate=2&burst=1", nil)
	w3 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w3, req3)

	retryAfter := w3.Header().Get("Retry-After")
	retrySeconds, _ := strconv.Atoi(retryAfter)

	// Wait for the suggested time
	time.Sleep(time.Duration(retrySeconds) * time.Second)

	// Next request should succeed
	req4, _ := http.NewRequest("GET", "/allow?key=test_timing&rate=2&burst=1", nil)
	w4 := httptest.NewRecorder()
	s.Handler.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("Expected request to succeed after waiting, got status %d", w4.Code)
	}
}
