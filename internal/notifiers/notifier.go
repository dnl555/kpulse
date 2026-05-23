// Package notifiers defines the Notifier interface and concrete sinks.
package notifiers

import (
	"context"

	"github.com/dnl555/kpulse/internal/alert"
)

type Notifier interface {
	Name() string
	Send(ctx context.Context, a alert.Alert) error
}
