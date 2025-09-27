package throttled

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type Server struct {
	Cache   *lru.Cache
	Handler http.Handler
}

func getOrGenerateRequestID(r *http.Request) string {
	if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
		return requestID
	}
	uuidV7, err := uuid.NewV7()
	if err != nil {
		// Fallback to v4 if v7 generation fails
		return uuid.NewString()
	}
	return uuidV7.String()
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

func NewServer(cacheSize int) (*Server, error) {
	cache, err := lru.NewWithEvict(cacheSize, func(key any, value any) {
		cacheEvictionsTotal.Inc()
		slog.Debug("Cache eviction", "key", key)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	s := &Server{
		Cache: cache,
	}

	initMetrics(cache)

	mux := http.NewServeMux()
	mux.HandleFunc("/allow", s.instrumentHandler("allow", s.allowHandleFunc))
	mux.HandleFunc("/wait", s.instrumentHandler("wait", s.waitHandleFunc))
	mux.Handle("/metrics", promhttp.Handler())
	s.Handler = mux
	return s, nil
}

type LimitRequest struct {
	Key   string
	Rate  rate.Limit
	Burst int
}

func parseLimitRequest(r *http.Request) (*LimitRequest, error) {
	key := r.FormValue("key")
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	rateLimit, err := strconv.ParseFloat(r.FormValue("rate"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid rate: %w", err)
	}
	burst, err := strconv.ParseInt(r.FormValue("burst"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid burst: %w", err)
	}
	return &LimitRequest{
		Key:   key,
		Rate:  rate.Limit(rateLimit),
		Burst: int(burst),
	}, nil
}

func (s *Server) getLimiter(rv *LimitRequest, requestID string) *rate.Limiter {
	if _l, ok := s.Cache.Get(rv.Key); ok {
		l := _l.(*rate.Limiter)
		if rv.Rate == l.Limit() && rv.Burst == l.Burst() {
			return l
		}
		// renew limiter when rate or burst parameters changed
		slog.Debug("Limiter renewed", "request_id", requestID, "key", rv.Key, "old_rate", l.Limit(), "new_rate", rv.Rate, "old_burst", l.Burst(), "new_burst", rv.Burst)
	}
	l := rate.NewLimiter(rv.Rate, rv.Burst)
	s.Cache.Add(rv.Key, l)
	rateLimitCreatedTotal.Inc()
	slog.Debug("Limiter created", "request_id", requestID, "key", rv.Key, "rate", rv.Rate, "burst", rv.Burst)
	return l
}

func (s *Server) allowHandleFunc(w http.ResponseWriter, r *http.Request) {
	requestID := getRequestID(r.Context())
	rv, err := parseLimitRequest(r)
	if err != nil {
		slog.Warn("Invalid request parameters", "request_id", requestID, "error", err, "query", r.URL.RawQuery)
		s.response(w, http.StatusBadRequest)
		return
	}
	slog.Debug("Request parameters parsed", "request_id", requestID, "key", rv.Key, "rate", rv.Rate, "burst", rv.Burst)

	l := s.getLimiter(rv, requestID)
	if l.Allow() {
		rateLimitAllowedTotal.Inc()
		slog.Debug("Rate limit check", "request_id", requestID, "key", rv.Key, "allowed", true)
		s.response(w, http.StatusOK)
		return
	} else {
		rateLimitHitsTotal.Inc()
		retryAfter := s.calculateRetryAfter(l)
		slog.Info("Rate limited", "request_id", requestID, "key", rv.Key, "endpoint", "/allow", "retry_after", retryAfter)
		slog.Debug("Rate limit check", "request_id", requestID, "key", rv.Key, "allowed", false)
		s.responseWithRetryAfter(w, http.StatusTooManyRequests, l)
		return
	}
}

func (s *Server) waitHandleFunc(w http.ResponseWriter, r *http.Request) {
	requestID := getRequestID(r.Context())
	rv, err := parseLimitRequest(r)
	if err != nil {
		slog.Warn("Invalid request parameters", "request_id", requestID, "error", err, "query", r.URL.RawQuery)
		s.response(w, http.StatusBadRequest)
		return
	}
	slog.Debug("Request parameters parsed", "request_id", requestID, "key", rv.Key, "rate", rv.Rate, "burst", rv.Burst)

	l := s.getLimiter(rv, requestID)
	start := time.Now()
	if err := l.Wait(r.Context()); err != nil {
		rateLimitHitsTotal.Inc()
		slog.Error("Wait operation cancelled", "request_id", requestID, "key", rv.Key, "error", err)
		s.response(w, http.StatusTooManyRequests)
		return
	} else {
		rateLimitAllowedTotal.Inc()
		waitDuration := time.Since(start).Seconds()
		if waitDuration > 0.001 { // Only log if wait was significant (>1ms)
			slog.Debug("Wait operation", "request_id", requestID, "key", rv.Key, "wait_duration", waitDuration)
		}
		s.response(w, http.StatusOK)
		return
	}
}

func (s *Server) response(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	fmt.Fprintln(w, http.StatusText(code))
}

func (s *Server) instrumentHandler(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate or get request ID
		requestID := getOrGenerateRequestID(r)
		ctx := withRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		start := time.Now()
		w2 := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		handler.ServeHTTP(w2, r)
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(endpoint).Observe(duration)
		requestsTotal.WithLabelValues(endpoint, fmt.Sprintf("%d", w2.statusCode)).Inc()

		// HTTP access log
		key := r.FormValue("key")
		rate := r.FormValue("rate")
		burst := r.FormValue("burst")
		slog.Info("HTTP request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"key", key,
			"rate", rate,
			"burst", burst,
			"status", w2.statusCode,
			"duration", duration,
			"remote_addr", r.RemoteAddr,
		)
	}
}

func (s *Server) calculateRetryAfter(limiter *rate.Limiter) int {
	r := limiter.Reserve()
	delay := r.Delay()
	r.Cancel() // Cancel the reservation since we don't actually want to reserve
	return int(math.Ceil(delay.Seconds()))
}

func (s *Server) responseWithRetryAfter(w http.ResponseWriter, code int, limiter *rate.Limiter) {
	w.Header().Set("Content-Type", "text/plain")

	retryAfterSeconds := s.calculateRetryAfter(limiter)
	if retryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}

	w.WriteHeader(code)
	fmt.Fprintln(w, http.StatusText(code))
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode    int
	headerWritten bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.headerWritten {
		return
	}
	r.statusCode = code
	r.headerWritten = true
	r.ResponseWriter.WriteHeader(code)
}
