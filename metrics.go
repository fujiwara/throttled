package throttled

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
	rateLimitHitsTotal    prometheus.Counter
	rateLimitAllowedTotal prometheus.Counter
	rateLimitCreatedTotal prometheus.Counter
	cacheEvictionsTotal   prometheus.Counter
	cacheSize             prometheus.GaugeFunc
	limitersActive        prometheus.GaugeFunc
)

func init() {
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "throttled_requests_total",
			Help: "Total number of requests processed by endpoint and status",
		},
		[]string{"endpoint", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "throttled_request_duration_seconds",
			Help:    "Request processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	rateLimitHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "throttled_rate_limit_hits_total",
			Help: "Total number of rate limit hits (429 responses)",
		},
	)

	rateLimitAllowedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "throttled_rate_limit_allowed_total",
			Help: "Total number of allowed requests",
		},
	)

	rateLimitCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "throttled_rate_limit_created_total",
			Help: "Total number of newly created rate limiters",
		},
	)

	cacheEvictionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "throttled_cache_evictions_total",
			Help: "Total number of cache evictions",
		},
	)
}

func initMetrics(cache *lru.Cache) {
	if cacheSize == nil {
		cacheSize = promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "throttled_cache_size",
				Help: "Current cache size",
			},
			func() float64 {
				if cache == nil {
					return 0
				}
				return float64(cache.Len())
			},
		)
	}

	if limitersActive == nil {
		limitersActive = promauto.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "throttled_limiters_active",
				Help: "Number of active rate limiters",
			},
			func() float64 {
				if cache == nil {
					return 0
				}
				return float64(cache.Len())
			},
		)
	}
}
