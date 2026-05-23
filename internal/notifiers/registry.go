package notifiers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/dnl555/kpulse/internal/alert"
)

type Registry struct {
	mu sync.RWMutex
	m  map[string]Notifier
}

func NewRegistry() *Registry { return &Registry{m: map[string]Notifier{}} }

func (r *Registry) Register(n Notifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[n.Name()] = n
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.m))
	for k := range r.m {
		out = append(out, k)
	}
	return out
}

func (r *Registry) Get(name string) (Notifier, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.m[name]
	return n, ok
}

func (r *Registry) Send(ctx context.Context, a alert.Alert, channels []string) error {
	if len(channels) == 0 {
		return nil
	}
	var errs []string
	for _, c := range channels {
		n, ok := r.Get(c)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown channel", c))
			continue
		}
		if err := n.Send(ctx, a); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
