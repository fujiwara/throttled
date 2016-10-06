package throttled_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fujiwara/throttled"
)

var ts *httptest.Server

func TestMain(m *testing.M) {
	ts = httptest.NewServer(throttled.Handler(ioutil.Discard))
	os.Exit(m.Run())
}

func TestAllow(t *testing.T) {
	for i := 0; i < 5; i++ {
		u := ts.URL + "/allow?key=test_allow&rate=1&burst=1&expires=60"
		res, err := http.Get(u)
		if err != nil {
			t.Errorf("GET %s error: %s", u, err)
			return
		}
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
		switch res.StatusCode {
		case http.StatusOK, http.StatusCreated:
		default:
			t.Errorf("GET %s status: %d", u, res.StatusCode)
		}
		time.Sleep(1 * time.Second)
	}
}

func TestAllowBurst(t *testing.T) {
	for i := 0; i < 10; i++ {
		u := ts.URL + "/allow?key=test_allow_burst&rate=1&burst=10&expires=60"
		res, err := http.Get(u)
		if err != nil {
			t.Errorf("GET %s error: %s", u, err)
			return
		}
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
		switch res.StatusCode {
		case http.StatusOK, http.StatusCreated:
		default:
			t.Errorf("GET %s status: %d", u, res.StatusCode)
		}
	}
}

func TestAllow429(t *testing.T) {
	s := make(map[int]int)
	for i := 0; i < 100; i++ {
		u := ts.URL + "/allow?key=test_allow_restricted&rate=1&burst=1&expires=60"
		res, err := http.Get(u)
		if err != nil {
			t.Errorf("GET %s error: %s", u, err)
			return
		}
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
		s[res.StatusCode]++
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("%#v", s)
	ok := s[http.StatusOK] + s[http.StatusCreated]
	if ok > 15 || ok < 10 {
		t.Errorf("2xx count is not expected %d", ok)
	}
	ng := s[http.StatusTooManyRequests]
	if ng < 85 || ng > 90 {
		t.Errorf("429 count is not expected %d", ng)
	}
	delete(s, http.StatusOK)
	delete(s, http.StatusCreated)
	delete(s, http.StatusTooManyRequests)
	if len(s) > 0 {
		t.Errorf("unexpeted status code %#v", s)
	}
}

func TestWait(t *testing.T) {
	var responseTime time.Duration
	var wg sync.WaitGroup
	var mu sync.Mutex
	start := time.Now()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			u := fmt.Sprintf("%s%s%d", ts.URL, "/wait?key=test_wait&rate=10&burst=10&expires=60&i=", i)
			s := time.Now()
			res, err := http.Get(u)
			if err != nil {
				t.Errorf("GET %s error: %s", u, err)
				return
			}
			io.Copy(ioutil.Discard, res.Body)
			res.Body.Close()
			if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
				t.Errorf("failed to request %s %d", u, res.StatusCode)
			}
			d := time.Now().Sub(s)
			//			t.Logf("resptime %d %s", i, d)
			mu.Lock()
			responseTime += d
			mu.Unlock()
			wg.Done()
		}(i)
	}
	wg.Wait()
	elapsed := time.Now().Sub(start)
	t.Logf("elapsed %s", elapsed)
	t.Logf("avarage %s", responseTime/100)
	if elapsed > time.Duration(12)*time.Second || elapsed < time.Duration(8)*time.Second {
		t.Errorf("elased time is not expected %s", elapsed)
	}
}
