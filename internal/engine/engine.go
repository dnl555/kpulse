package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type Options struct {
	Dedupe           *Deduper
	Router           *Router
	Registry         *notifiers.Registry
	Cluster          string
	DigestEnabled    bool
	DigestInterval   time.Duration
	DigestSeverities []alert.Severity
}

type Engine struct {
	opts    Options
	in      chan alert.Alert
	digestQ []alert.Alert
	mu      sync.Mutex
}

func New(o Options) *Engine {
	return &Engine{opts: o, in: make(chan alert.Alert, 256)}
}

func (e *Engine) Submit(a alert.Alert) {
	a.EnsureFiredAt()
	if a.Cluster == "" {
		a.Cluster = e.opts.Cluster
	}
	select {
	case e.in <- a:
	default:
	}
}

func (e *Engine) Run(ctx context.Context) {
	var tickC <-chan time.Time
	if e.opts.DigestEnabled && e.opts.DigestInterval > 0 {
		t := time.NewTicker(e.opts.DigestInterval)
		defer t.Stop()
		tickC = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-e.in:
			e.handle(ctx, a)
		case <-tickC:
			e.flushDigest(ctx)
		}
	}
}

func (e *Engine) handle(ctx context.Context, a alert.Alert) {
	if !e.opts.Dedupe.Allow(a) {
		return
	}
	if e.opts.DigestEnabled && containsSev(e.opts.DigestSeverities, a.Severity) {
		e.mu.Lock()
		e.digestQ = append(e.digestQ, a)
		e.mu.Unlock()
		return
	}
	channels := e.opts.Router.Channels(a)
	_ = e.opts.Registry.Send(ctx, a, channels)
}

func (e *Engine) flushDigest(ctx context.Context) {
	e.mu.Lock()
	batch := e.digestQ
	e.digestQ = nil
	e.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d alerts in the last interval:\n\n", len(batch))
	for _, a := range batch {
		fmt.Fprintf(&b, "- [%s] %s/%s %s: %s\n", a.Severity.String(), a.Namespace, a.Object(), a.Reason, a.Title)
	}
	digest := alert.Alert{
		Monitor: "digest", Severity: alert.Info, Cluster: e.opts.Cluster,
		Title: fmt.Sprintf("%d alerts (digest)", len(batch)), Body: b.String(),
		FiredAt: time.Now().UTC(),
	}
	channels := e.opts.Router.Channels(digest)
	_ = e.opts.Registry.Send(ctx, digest, channels)
}

func containsSev(list []alert.Severity, s alert.Severity) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
