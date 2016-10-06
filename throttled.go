package throttled

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	accesslog "github.com/mash/go-accesslog"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

var (
	mux       = http.NewServeMux()
	maxWait   = 10 * time.Second
	throttles *lru.Cache
	evicted   int64
	mu        sync.Mutex
	logMu     sync.Mutex
	stats     *Stats
)

type logger struct {
	w       io.Writer
	encoder *json.Encoder
}

type LogRecord struct {
	Time        time.Time `json:"time"`
	Ip          string    `json:"host"`
	Method      string    `json:"method"`
	Uri         string    `json:"uri"`
	Protocol    string    `json:"protocol"`
	Key         string    `json:"key"`
	Host        string    `json:"vhost"`
	Status      int       `json:"status"`
	Size        int64     `json:"size"`
	ElapsedTime Apptime   `json:"apptime"`
}

type Stats struct {
	Size      int       `json:"cache_size"`
	Keys      int       `json:"keys"`
	Evicted   int64     `json:"evicted"`
	Created   int64     `json:"created"`
	Passed    int64     `json:"passed"`
	Throttled int64     `json:"throttled"`
	Uptime    float64   `json:"uptime"`
	Started   time.Time `json:"started"`
}

func (s *Stats) Update() {
	s.Keys = throttles.Len()
	s.Uptime = time.Now().Sub(s.Started).Seconds()
}

func (s *Stats) Allow() {
	mu.Lock()
	defer mu.Unlock()
	s.Passed++
}

func (s *Stats) Deny() {
	mu.Lock()
	defer mu.Unlock()
	s.Throttled++
}

func (s *Stats) Create() {
	mu.Lock()
	defer mu.Unlock()
	s.Created++
	s.Passed++
}

func (s *Stats) Evict(k, v interface{}) {
	mu.Lock()
	defer mu.Unlock()
	s.Evicted++
}

type Apptime struct {
	time.Duration
}

func (t Apptime) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.6f", t.Seconds())
	return []byte(s), nil
}

func (l logger) Log(r accesslog.LogRecord) {
	logMu.Lock()
	defer logMu.Unlock()
	l.encoder.Encode(LogRecord{
		Time:        r.Time,
		Ip:          r.Ip,
		Method:      r.Method,
		Uri:         r.Uri,
		Protocol:    r.Protocol,
		Key:         r.CustomRecords["Key"],
		Host:        r.Host,
		Status:      r.Status,
		Size:        r.Size,
		ElapsedTime: Apptime{r.ElapsedTime},
	})
}

func (l logger) Flush() error {
	if _w, ok := l.w.(Flusher); ok {
		logMu.Lock()
		defer logMu.Unlock()
		return _w.Flush()
	}
	return nil
}

func (l logger) PeriodicalFlush() {
	c := time.Tick(1 * time.Second)
	for range c {
		err := l.Flush()
		if err != nil {
			log.Println(err)
		}
	}
}

type Flusher interface {
	Flush() error
}

type Request struct {
	Key   string
	Rate  rate.Limit
	Burst int
}

func init() {
	mux.HandleFunc("/allow", allowHandler)
	mux.HandleFunc("/wait", waitHandler)
	mux.HandleFunc("/stats", statsHandler)
}

func Setup(size int) {
	var err error
	stats = &Stats{
		Size:    size,
		Started: time.Now(),
	}
	throttles, err = lru.NewWithEvict(size, stats.Evict)
	if err != nil {
		panic(err)
	}
}

func Handler(w io.Writer) http.Handler {
	if w == nil {
		return mux
	}

	l := logger{
		w:       w,
		encoder: json.NewEncoder(w),
	}
	go l.PeriodicalFlush()
	return accesslog.NewLoggingHandler(mux, l)
}

func parseRequest(r *http.Request) (*Request, error) {
	key := r.FormValue("key")
	if key == "" {
		return nil, errors.New("invalid key")
	}
	rateLimit, err := strconv.ParseFloat(r.FormValue("rate"), 64)
	if err != nil {
		return nil, errors.New("invalid rate")
	}
	burst, err := strconv.ParseInt(r.FormValue("burst"), 10, 64)
	if err != nil {
		return nil, errors.New("invalid burst")
	}
	return &Request{
		Key:   key,
		Rate:  rate.Limit(rateLimit),
		Burst: int(burst),
	}, nil
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	rv, err := parseRequest(r)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}
	l := rate.NewLimiter(rv.Rate, rv.Burst)
	throttles.Add(rv.Key, l)
	stats.Create()
	response(w, http.StatusCreated)
}

func allowHandler(w http.ResponseWriter, r *http.Request) {
	rv, err := parseRequest(r)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}
	if _w, ok := w.(*accesslog.LoggingWriter); ok {
		_w.SetCustomLogRecord("Key", rv.Key)
	}

	if _l, ok := throttles.Get(rv.Key); ok {
		l := _l.(*rate.Limiter)
		if rv.Rate != l.Limit() || rv.Burst != l.Burst() {
			// renew a limiter
			setHandler(w, r)
			return
		}
		if l.Allow() {
			stats.Allow()
			response(w, http.StatusOK)
		} else {
			stats.Deny()
			response(w, http.StatusTooManyRequests)
		}
		return
	}
	setHandler(w, r)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	stats.Update()
	enc.Encode(stats)
}

func waitHandler(w http.ResponseWriter, r *http.Request) {
	rv, err := parseRequest(r)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}
	if _w, ok := w.(*accesslog.LoggingWriter); ok {
		_w.SetCustomLogRecord("Key", rv.Key)
	}

	if _l, ok := throttles.Get(rv.Key); ok {
		l := _l.(*rate.Limiter)
		if rv.Rate != l.Limit() || rv.Burst != l.Burst() {
			// renew a limiter
			setHandler(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), maxWait)
		defer cancel()
		if err := l.Wait(ctx); err != nil {
			stats.Deny()
			response(w, http.StatusTooManyRequests)
		} else {
			stats.Allow()
			response(w, http.StatusOK)
		}
		return
	}
	setHandler(w, r)
}

func response(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	fmt.Fprintln(w, http.StatusText(code))
}
