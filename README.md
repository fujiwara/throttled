# throttled

A throttling httpd by Go.

## Install & Run

```
go get github.com/fujiwara/throttled/cmd/throtted
```

```
$ throtted -port 8888
```

`-port` is required.

## API

`throttled` uses `golang.org/x/time/rate` for throttling.

`x/time/rate` implements a "token bucket" rate limitter.

### /allow

```
GET /allow?key=${identifier}&rate=${rate}&burst=${burst}&expires=${expires}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)
- expires: Expiration time(sec) of a rate limiter for each `key`(int)

`/allow` returns a response immediately.

- 200: OK. Allowed by a rate limiter.
- 201: A rate limiter for `key` was created.
- 429: Not allowed by a rate limiter.

### /wait

```
GET /wait?key=${identifier}&rate=${rate}&burst=${burst}&expires=${expires}
```

- rate: Request rate(/sec) (float)
- burst: Burst tokens count (int)
- expires: Expiration time(sec) of a rate limiter for each `key`(int)

If a request is not allowed `/wait` waits until allowed, and returns a response.

- 200: OK. Allowed by a rate limiter.
- 201: A rate limiter for `key` was created.
- 429: Not allowed by a rate limiter.

## LICENSE

The MIT License (MIT)

Copyright (c) 2016 FUJIWARA Shunichiro
