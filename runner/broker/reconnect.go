package broker

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// ReconnectConfig controls the backoff policy used by Run.
type ReconnectConfig struct {
	// URL is the amqp:// URL to dial.
	URL string
	// MinBackoff is the starting wait after a failed attempt.
	// Defaults to 100ms when zero.
	MinBackoff time.Duration
	// MaxBackoff caps the backoff.  Defaults to 30s when zero.
	MaxBackoff time.Duration
	// Logger, when non-nil, receives structured logs on connect
	// attempts and failures.
	Logger *slog.Logger
}

// ErrHandlerDone indicates the handler decided to stop voluntarily
// (e.g. ctx was cancelled).  Run treats it as a clean shutdown.
var ErrHandlerDone = errors.New("broker handler done")

// Run dials the broker and invokes handler with the resulting Conn.
// If handler returns an error other than ErrHandlerDone, Run closes
// the connection, waits an exponentially backed-off interval, and
// redials — until the given ctx is cancelled.
//
// Handler is expected to return whenever ctx is cancelled; Run does
// not forcibly cancel the handler's work beyond what the context
// carries.
func Run(ctx context.Context, cfg ReconnectConfig, handler func(context.Context, *Conn) error) error {
	min := cfg.MinBackoff
	if min <= 0 {
		min = 100 * time.Millisecond
	}
	max := cfg.MaxBackoff
	if max <= 0 {
		max = 30 * time.Second
	}
	backoff := min
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := Dial(ctx, cfg.URL)
		if err != nil {
			logger.Warn("broker dial failed, backing off", "err", err, "backoff", backoff)
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, max)
			continue
		}
		backoff = min // reset on successful connect
		herr := handler(ctx, conn)
		_ = conn.Close(context.Background())
		if errors.Is(herr, ErrHandlerDone) || ctx.Err() != nil {
			return ctx.Err()
		}
		if herr != nil {
			logger.Warn("broker handler exited with error, reconnecting", "err", herr, "backoff", backoff)
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, max)
		}
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	next := cur * 2
	if next > max {
		return max
	}
	return next
}

// sleep waits for d or until ctx is done.  Returns false if the context
// was cancelled before d elapsed.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
