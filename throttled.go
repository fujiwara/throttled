package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	cache "github.com/patrickmn/go-cache"
	"golang.org/x/time/rate"
)

var (
	throttles = cache.New(5*time.Minute, 120*time.Second)
)

func init() {
	http.HandleFunc("/", allowHandler)
}

func main() {
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setHandler(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		response(w, http.StatusBadRequest)
		return
	}
	limit, err := strconv.ParseFloat(r.FormValue("limit"), 64)
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

	//	log.Printf("Set %s rate:%f burst:%d expires:%ds", key, limit, burst, expires)
	l := rate.NewLimiter(rate.Limit(limit), int(burst))
	throttles.Set(key, l, time.Duration(expires)*time.Second)
	response(w, http.StatusCreated)
}

func allowHandler(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if key == "" {
		response(w, http.StatusBadRequest)
		return
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

func response(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(code)
	fmt.Fprintln(w, http.StatusText(code))
}
