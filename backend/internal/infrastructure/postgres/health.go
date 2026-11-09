package postgres

import (
	"context"
	"time"
)

type pinger interface {
	Ping(context.Context) error
}

// Health verifies database availability without exposing connection details.
type Health struct {
	pool    pinger
	timeout time.Duration
}

func NewHealth(pool pinger, timeout time.Duration) *Health {
	return &Health{pool: pool, timeout: timeout}
}

func (health *Health) Check(ctx context.Context) error {
	checkContext, cancel := context.WithTimeout(ctx, health.timeout)
	defer cancel()
	return health.pool.Ping(checkContext)
}
