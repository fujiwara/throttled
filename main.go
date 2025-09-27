package throttled

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

func Run(ctx context.Context) error {
	cli := CLI{}
	kong.Parse(&cli)

	// Initialize slog
	level := parseLogLevel(cli.LogLevel)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("Server starting", "port", cli.Port, "cache_size", cli.CacheSize, "log_level", cli.LogLevel)

	s, err := NewServer(cli.CacheSize)
	if err != nil {
		slog.Error("Failed to create server", "error", err)
		return fmt.Errorf("failed to create server: %w", err)
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cli.Port),
		Handler: s.Handler,
	}
	// Shutdown server gracefully
	go func() {
		<-ctx.Done()
		slog.Info("Server shutting down gracefully")
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx2); err != nil {
			slog.Error("Error during server shutdown", "error", err)
		} else {
			slog.Info("Server shutdown completed")
		}
	}()
	return srv.ListenAndServe()
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
