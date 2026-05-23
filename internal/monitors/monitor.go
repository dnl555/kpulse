// Package monitors defines the Monitor interface and concrete monitor implementations.
package monitors

import (
	"context"

	"github.com/dnl555/kpulse/internal/alert"
)

type Submitter interface {
	Submit(alert.Alert)
}

type Monitor interface {
	Name() string
	Run(ctx context.Context, sub Submitter) error
}
