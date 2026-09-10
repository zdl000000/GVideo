package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) PingContext(ctx context.Context) error { return f(ctx) }

func TestReadiness(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		checker := NewReadiness(pingerFunc(func(context.Context) error { return nil }))
		if err := checker.Ready(context.Background()); err != nil {
			t.Fatalf("Ready() error = %v", err)
		}
	})

	t.Run("ping error", func(t *testing.T) {
		want := errors.New("database unavailable")
		checker := NewReadiness(pingerFunc(func(context.Context) error { return want }))
		if err := checker.Ready(context.Background()); !errors.Is(err, want) {
			t.Fatalf("Ready() error = %v, want %v", err, want)
		}
	})

	t.Run("one second upper bound", func(t *testing.T) {
		checker := NewReadiness(pingerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}))
		started := time.Now()
		if err := checker.Ready(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Ready() error = %v, want deadline exceeded", err)
		}
		elapsed := time.Since(started)
		if elapsed < 900*time.Millisecond || elapsed > 1500*time.Millisecond {
			t.Fatalf("Ready() elapsed = %s, want approximately one second", elapsed)
		}
	})

	t.Run("shorter caller deadline", func(t *testing.T) {
		checker := NewReadiness(pingerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		started := time.Now()
		if err := checker.Ready(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Ready() error = %v, want deadline exceeded", err)
		}
		if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
			t.Fatalf("Ready() ignored caller deadline; elapsed = %s", elapsed)
		}
	})

	t.Run("nil database", func(t *testing.T) {
		if err := NewReadiness(nil).Ready(context.Background()); !errors.Is(err, errDatabaseNotConfigured) {
			t.Fatalf("Ready() error = %v, want configuration error", err)
		}
	})
}
