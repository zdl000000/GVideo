package health

import (
	"context"
	"errors"
	"time"
)

const readinessTimeout = time.Second

var errDatabaseNotConfigured = errors.New("readiness database is not configured")

// Pinger is the narrow database capability needed to establish readiness.
type Pinger interface {
	PingContext(context.Context) error
}

// Readiness reports whether the database initialized by the server remains
// reachable. It deliberately excludes workers, queues, media, and commands.
type Readiness struct {
	database Pinger
	timeout  time.Duration
}

func NewReadiness(database Pinger) *Readiness {
	return &Readiness{database: database, timeout: readinessTimeout}
}

func (r *Readiness) Ready(ctx context.Context) error {
	if r == nil || r.database == nil {
		return errDatabaseNotConfigured
	}
	checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.database.PingContext(checkCtx)
}
