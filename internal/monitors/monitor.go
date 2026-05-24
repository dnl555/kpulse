// Package monitors defines the Monitor interface and concrete monitor implementations.
package monitors

import (
	"context"

	"github.com/dnl555/kpulse/internal/alert"
)

type Submitter interface {
	Submit(alert.Alert)
	// Resolve marks an alert as resolved. The Alert struct only needs the key
	// fields (Monitor, Namespace, ObjectKind, ObjectName, Reason) to match an
	// active firing alert; Title/Body are used as the resolved message.
	Resolve(alert.Alert)
	// Reconcile is used by periodic monitors. It declares the complete set of
	// currently-firing alerts for the given monitor; the engine submits each
	// one and resolves any previously active alert for that monitor not in the
	// list.
	Reconcile(monitor string, firing []alert.Alert)
}

type Monitor interface {
	Name() string
	Run(ctx context.Context, sub Submitter) error
}
