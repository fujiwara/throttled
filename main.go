package throttled

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alecthomas/kong"
)

func Run(ctx context.Context) error {
	cli := CLI{}
	kong.Parse(&cli)

	s, err := NewServer(cli.CacheSize)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cli.Port),
		Handler: s.Handler,
	}
	// Shutdown server gracefully
	go func() {
		<-ctx.Done()
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx2)
	}()
	return srv.ListenAndServe()
}
