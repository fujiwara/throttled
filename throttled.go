package throttled

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	accesslog "github.com/mash/go-accesslog"
	cache "github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

var (
	mux       = http.NewServeMux()
	throttles = cache.New(5*time.Minute, 30*time.Second)
	maxWait   = 10 * time.Second
)

type logger struct {
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

type Apptime struct {
	time.Duration
}

func (t Apptime) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf("%.6f", t.Seconds())
	return []byte(s), nil
}

func (l logger) Log(r accesslog.LogRecord) {
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

func init() {
	mux.HandleFunc("/allow", allowHandler)
	mux.HandleFunc("/wait", waitHandler)
}

func Handler(w io.Writer) http.Handler {
	l := logger{
		encoder: json.NewEncoder(w),
	}
	return accesslog.NewLoggingHandler(mux, l)
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		response(w, http.StatusBadRequest)
		return
	}
	rateLimit, err := strconv.ParseFloat(r.FormValue("rate"), 64)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}
	burst, err := strconv.ParseInt(r.FormValue("burst"), 10, 64)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}
	expires, err := strconv.ParseInt(r.FormValue("expires"), 10, 64)
	if err != nil {
		response(w, http.StatusBadRequest)
		return
	}

	l := rate.NewLimiter(rate.Limit(rateLimit), int(burst))
	throttles.Set(key, l, time.Duration(expires)*time.Second)
	response(w, http.StatusCreated)
}

func allowHandler(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		response(w, http.StatusBadRequest)
		return
	}
	if _w, ok := w.(*accesslog.LoggingWriter); ok {
		_w.SetCustomLogRecord("Key", key)
	}

	if l, ok := throttles.Get(key); ok {
		if l.(*rate.Limiter).Allow() {
			response(w, http.StatusOK)
		} else {
			response(w, http.StatusTooManyRequests)
		}
		return
	}
	setHandler(w, r)
}

func waitHandler(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		response(w, http.StatusBadRequest)
		return
	}
	if _w, ok := w.(*accesslog.LoggingWriter); ok {
		_w.SetCustomLogRecord("Key", key)
	}

	if l, ok := throttles.Get(key); ok {
		ctx, cancel := context.WithTimeout(context.Background(), maxWait)
		defer cancel()
		if err := l.(*rate.Limiter).Wait(ctx); err != nil {
			response(w, http.StatusTooManyRequests)
		} else {
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
