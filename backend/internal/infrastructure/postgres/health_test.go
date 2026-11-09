package postgres

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubPinger struct{ err error }

func (stub stubPinger) Ping(context.Context) error { return stub.err }

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	if err := NewHealth(stubPinger{}, time.Second).Check(context.Background()); err != nil {
		t.Fatalf("expected healthy database, got %v", err)
	}

	expected := errors.New("database unavailable")
	if err := NewHealth(stubPinger{err: expected}, time.Second).Check(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("expected database error, got %v", err)
	}
}
