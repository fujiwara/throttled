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

### Try `/allow` endpoint with [vegeta](https://github.com/tsenart/vegeta).

#### Send 50 req/sec for 10 seconds.

- All requests are allowed because the rate limit is high enough (100 req/sec).

```console
$ echo "GET http://localhost:8000/allow?key=foo&rate=100&burst=100" \
  | vegeta attack -rate=50/s -duration=10s \
  | vegeta report
Requests      [total, rate, throughput]  500, 50.10, 50.09
Duration      [total, attack, wait]      9.981370923s, 9.980667035s, 703.888µs
Latencies     [mean, 50, 95, 99, max]    755.588µs, 732.06µs, 974.465µs, 1.314326ms, 1.875267ms
Bytes In      [total, mean]              1500, 3.00
Bytes Out     [total, mean]              0, 0.00
Success       [ratio]                    100.00%
Status Codes  [code:count]               200:500
```

#### Send 150 req/sec for 10 seconds.

- Some requests are rate-limited because the rate limit is 100 req/sec and the burst is 100.

```
$ echo "GET http://localhost:8000/allow?key=foo&rate=100&burst=100" \
  | vegeta attack -rate=150/s -duration=10s \
  | vegeta report
Requests      [total, rate, throughput]  1500, 150.10, 109.97
Duration      [total, attack, wait]      9.993624552s, 9.993018463s, 606.089µs
Latencies     [mean, 50, 95, 99, max]    638.554µs, 622.971µs, 822.782µs, 996.477µs, 1.793067ms
Bytes In      [total, mean]              10515, 7.01
Bytes Out     [total, mean]              0, 0.00
Success       [ratio]                    73.27%
Status Codes  [code:count]               200:1099  429:401
Error Set:
429 Too Many Requests
```

### Try `/wait` endpoint with [vegeta](https://github.com/tsenart/vegeta).

#### Send 50 req/sec for 10 seconds.

- All requests are allowed.
- All requests are processed quickly because the rate limit is high enough (100 req/sec).

```console
$ echo "GET http://localhost:8000/wait?key=foo&rate=100&burst=100" \
  | vegeta attack -rate=50/s -duration=10s \
  | vegeta report
Requests      [total, rate, throughput]  500, 50.10, 50.10
Duration      [total, attack, wait]      9.980926429s, 9.980267941s, 658.488µs
Latencies     [mean, 50, 95, 99, max]    783.105µs, 743.842µs, 1.077636ms, 1.277127ms, 2.447456ms
Bytes In      [total, mean]              1500, 3.00
Bytes Out     [total, mean]              0, 0.00
Success       [ratio]                    100.00%
Status Codes  [code:count]               200:500
```

#### Send 150 req/sec for 10 seconds.

- All requests are eventually allowed because `/wait` waits until allowed.
- Some requests take longer to be processed due to waiting.
- `throughput` is near the rate limit (100 req/sec).

```
$ echo "GET http://localhost:8000/wait?key=foo&rate=100&burst=100" \
  | vegeta attack -rate=150/s -duration=10s \
  | vegeta report
Requests      [total, rate, throughput]  1500, 150.10, 107.13
Duration      [total, attack, wait]      14.002010846s, 9.993653515s, 4.008357331s
Latencies     [mean, 50, 95, 99, max]    1.608736286s, 1.510623673s, 3.760693909s, 3.960715362s, 4.008357331s
Bytes In      [total, mean]              4500, 3.00
Bytes Out     [total, mean]              0, 0.00
Success       [ratio]                    100.00%
Status Codes  [code:count]               200:1500
```

## LICENSE

The MIT License (MIT)

Copyright (c) 2016 FUJIWARA Shunichiro
