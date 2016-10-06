# throttled

A throttling httpd by Go.

## Install & Run

```
go get github.com/fujiwara/throttled/cmd/throttled
```

```
$ throtted -port PORT [-size CACHE_SIZE]
```

- `-port` Listen port number. required.
- `-size` LRU cache size. optional. (default 100,000)

## API

`throttled` uses `golang.org/x/time/rate` for throttling.

`x/time/rate` implements a "token bucket" rate limitter.

### /allow

```
GET /allow?key=${identifier}&rate=${rate}&burst=${burst}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)

`/allow` returns a response immediately.

- 200: OK. Allowed by a rate limiter.
- 201: A rate limiter for `key` was created.
- 429: Not allowed by a rate limiter.

### /wait

```
GET /wait?key=${identifier}&rate=${rate}&burst=${burst}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)

If a request is not allowed `/wait` waits until allowed, and returns a response.

- 200: OK. Allowed by a rate limiter.
- 201: A rate limiter for `key` was created.
- 429: Not allowed by a rate limiter.

## Examples

```
$ wrk -c 10 -t 4 -d 10 "http://localhost:8888/allow?key=foo&rate=100&burst=100"
Running 10s test @ http://localhost:8888/allow?key=foo&rate=100&burst=100
  4 threads and 10 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   274.56us  838.09us  32.03ms   97.65%
    Req/Sec    10.64k     0.99k   13.59k    75.68%
  426613 requests in 10.10s, 54.89MB read
  Non-2xx or 3xx responses: 425502
Requests/sec:  42236.34
Transfer/sec:      5.43MB
```

426613(total) - 425502(Non-2xx or 3xx) = 1111 / 10.10s =~ 110 req/sec

```
$ wrk -c 10 -t 4 -d 10 "http://localhost:8888/wait?key=foo&rate=100&burst=100"
Running 10s test @ http://localhost:8888/wait?key=foo&rate=100&burst=100
  4 threads and 10 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency    72.43ms   23.23ms  82.63ms   90.40%
    Req/Sec    27.33     25.79   290.00     98.99%
  1104 requests in 10.05s, 112.13KB read
Requests/sec:    109.82
Transfer/sec:     11.15KB
```

## LICENSE

The MIT License (MIT)

Copyright (c) 2016 FUJIWARA Shunichiro
