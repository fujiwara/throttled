package throttled

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

type Server struct {
	Cache   *lru.Cache
	Handler http.Handler
}

func NewServer(cacheSize int) (*Server, error) {
	cache, err := lru.NewWithEvict(cacheSize, func(key any, value any) {
		cacheEvictionsTotal.Inc()
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

func (s *Server) newLimiter(rv *LimitRequest) {
	l := rate.NewLimiter(rv.Rate, rv.Burst)
	s.Cache.Add(rv.Key, l)
	rateLimitCreatedTotal.Inc()
}

func (s *Server) runLimiter(w http.ResponseWriter, r *http.Request, wait bool) {
	rv, err := parseLimitRequest(r)
	if err != nil {
		s.response(w, http.StatusBadRequest)
		return
	}

	if _l, ok := s.Cache.Get(rv.Key); !ok {
		s.newLimiter(rv)
		s.response(w, http.StatusCreated)
		return
	} else {
		l := _l.(*rate.Limiter)
		if rv.Rate != l.Limit() || rv.Burst != l.Burst() {
			// renew a limiter
			s.newLimiter(rv)
			s.response(w, http.StatusCreated)
		}
		if wait {
			if err := l.Wait(r.Context()); err != nil {
				rateLimitHitsTotal.Inc()
				s.response(w, http.StatusTooManyRequests)
			} else {
				rateLimitAllowedTotal.Inc()
				s.response(w, http.StatusOK)
			}
		} else {
			if l.Allow() {
				rateLimitAllowedTotal.Inc()
				s.response(w, http.StatusOK)
			} else {
				rateLimitHitsTotal.Inc()
				s.response(w, http.StatusTooManyRequests)
			}
		}
	}
}

func (s *Server) allowHandleFunc(w http.ResponseWriter, r *http.Request) {
	s.runLimiter(w, r, false)
}

func (s *Server) waitHandleFunc(w http.ResponseWriter, r *http.Request) {
	s.runLimiter(w, r, true)
}

func (s *Server) response(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	fmt.Fprintln(w, http.StatusText(code))
}

func (s *Server) instrumentHandler(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w2 := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		handler.ServeHTTP(w2, r)
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(endpoint).Observe(duration)
		requestsTotal.WithLabelValues(endpoint, fmt.Sprintf("%d", w2.statusCode)).Inc()
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
