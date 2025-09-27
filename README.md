# throttled

A throttling httpd by Go.

## Install & Run

```
go install github.com/fujiwara/throttled/cmd/throttled
```

```
$ throttled --port PORT [--size CACHE_SIZE]
```

- `--port` Listen port number. required.
- `--size` LRU cache size. optional. (default 100,000)

## API

`throttled` uses `golang.org/x/time/rate` for throttling.

`x/time/rate` implements a "token bucket" rate limiter.

**Note about burst parameter:**
- `burst` represents the maximum number of tokens that can be stored in the bucket
- When `burst=0`, no tokens can be stored, effectively blocking all requests immediately
- For practical use, set `burst` to at least 1

### /allow

```
GET /allow?key=${identifier}&rate=${rate}&burst=${burst}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)

`/allow` returns a response immediately.

- 200: OK. Allowed by a rate limiter.
- 429: Not allowed by a rate limiter. Includes `Retry-After` header with suggested wait time in seconds.

### /wait

```
GET /wait?key=${identifier}&rate=${rate}&burst=${burst}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)

If a request is not allowed `/wait` waits until allowed, and returns a response.

- 200: OK. Allowed by a rate limiter.
- 429: Not allowed by a rate limiter (context timeout). Includes `Retry-After` header with suggested wait time in seconds.

### /metrics

```
GET /metrics
```

Prometheus metrics endpoint that exposes the following metrics:

- `throttled_requests_total`: Total number of requests processed by endpoint and status
- `throttled_request_duration_seconds`: Request processing duration histogram
- `throttled_rate_limit_hits_total`: Total number of rate limit hits (429 responses)
- `throttled_rate_limit_allowed_total`: Total number of allowed requests
- `throttled_rate_limit_created_total`: Total number of newly created rate limiters
- `throttled_cache_size`: Current cache size
- `throttled_cache_evictions_total`: Total number of cache evictions
- `throttled_limiters_active`: Number of active rate limiters

## Examples

Try `/allow` endpoint with `wrk`:

```
$ wrk -c 10 -t 4 -d 10 "http://localhost:8000/allow?key=foo&rate=100&burst=100"
Running 10s test @ http://localhost:8000/allow?key=foo&rate=100&burst=100
  4 threads and 10 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   214.22us  151.33us   4.42ms   90.35%
    Req/Sec     9.26k   663.14    10.24k    78.96%
  372198 requests in 10.10s, 53.90MB read
  Non-2xx or 3xx responses: 372098
Requests/sec:  36849.90
Transfer/sec:      5.34MB
```

372198(total) - 372098(Non-2xx or 3xx) = 100 requests allowed.

Try `/wait` endpoint with `wrk`:

```
$ wrk -c 10 -t 4 -d 10 "http://localhost:8000/wait?key=foo&rate=100&burst=100"
Running 10s test @ http://localhost:8000/wait?key=foo&rate=100&burst=100
  4 threads and 10 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    72.48ms   23.15ms  81.14ms   90.46%
    Req/Sec    27.53     25.74   280.00     99.00%
  1101 requests in 10.01s, 111.82KB read
Requests/sec:    109.96
Transfer/sec:     11.17KB
```

Requests are throttled to about 100 req/sec. (first 100 requests are allowed immediately by burst=100)

## LICENSE

The MIT License (MIT)

Copyright (c) 2016 FUJIWARA Shunichiro
