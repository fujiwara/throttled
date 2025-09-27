# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

throttled is a rate-limiting HTTP server written in Go that implements token bucket rate limiting using `golang.org/x/time/rate`. It provides two endpoints (`/allow` and `/wait`) for immediate and blocking rate limit checks.

## Common Commands

### Build
```bash
make throttled           # Build the binary
go build -o throttled ./cmd/throttled  # Alternative build command
```

### Test
```bash
make test               # Run all tests with verbose output
go test -v ./...        # Alternative test command
go test -v -run TestAllow  # Run a specific test
```

### Install
```bash
make install            # Install the throttled command
go install github.com/fujiwara/throttled/cmd/throttled
```

### Clean
```bash
make clean              # Remove built binaries and dist directory
```

### Format
```bash
go fmt ./...            # Format all Go code (required before commits)
```

## Architecture

### Core Components

1. **Server (`throttled.go`)**: Main server implementation with LRU cache for rate limiters
   - Uses `hashicorp/golang-lru` for caching rate limiters
   - Implements `/allow` (non-blocking) and `/wait` (blocking) endpoints
   - Automatically renews limiters when rate/burst parameters change

2. **CLI (`cli.go`)**: Command-line interface using Kong parser
   - `--port`: Required listen port
   - `--size`: LRU cache size (default: 100,000)

3. **Entry Points**:
   - `cmd/throttled/main.go`: Binary entry point with signal handling
   - `main.go`: Core application runner with graceful shutdown

### Request Flow

1. Client sends request to `/allow` or `/wait` with parameters:
   - `key`: Identifier for rate limiting
   - `rate`: Requests per second (float)
   - `burst`: Token bucket burst size (int)

2. Server checks/creates rate limiter in LRU cache
3. Returns appropriate HTTP status:
   - 200: Request allowed
   - 201: New limiter created
   - 429: Rate limit exceeded (only for `/allow`)

### Key Design Decisions

- Rate limiters are stored in an LRU cache to manage memory usage
- Limiters are automatically renewed when parameters change
- `/wait` blocks until the request can be allowed (respects context cancellation)
- `/allow` returns immediately with success or rate limit error