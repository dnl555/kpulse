package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/notifiers"
)

type Options struct {
	Dedupe            *Deduper
	Router            *Router
	Registry          *notifiers.Registry
	Cluster           string
	DigestEnabled     bool
	DigestInterval    time.Duration
	DigestSeverities  []alert.Severity
	ResolutionEnabled bool
}

type inputKind int

const (
	inputFire inputKind = iota
	inputResolve
	inputReconcile
)

type input struct {
	kind    inputKind
	alert   alert.Alert
	monitor string
	firing  []alert.Alert
}

type Engine struct {
	opts    Options
	in      chan input
	digestQ []alert.Alert

	mu     sync.Mutex
	active map[string]alert.Alert // key -> last-known firing alert
}

func New(o Options) *Engine {
	return &Engine{
		opts:   o,
		in:     make(chan input, 256),
		active: map[string]alert.Alert{},
	}
}

func (e *Engine) Submit(a alert.Alert) {
	a.EnsureFiredAt()
	if a.Cluster == "" {
		a.Cluster = e.opts.Cluster
	}
	a.State = alert.StateFiring
	select {
	case e.in <- input{kind: inputFire, alert: a}:
	default:
	}
}

func (e *Engine) Resolve(a alert.Alert) {
	a.EnsureFiredAt()
	if a.Cluster == "" {
		a.Cluster = e.opts.Cluster
	}
	a.State = alert.StateResolved
	select {
	case e.in <- input{kind: inputResolve, alert: a}:
	default:
	}
}

func (e *Engine) Reconcile(monitor string, firing []alert.Alert) {
	cp := make([]alert.Alert, len(firing))
	for i, a := range firing {
		a.EnsureFiredAt()
		if a.Cluster == "" {
			a.Cluster = e.opts.Cluster
		}
		a.State = alert.StateFiring
		cp[i] = a
	}
	select {
	case e.in <- input{kind: inputReconcile, monitor: monitor, firing: cp}:
	default:
	}
}

func (e *Engine) ActiveSnapshot() map[string]alert.Alert {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]alert.Alert, len(e.active))
	for k, v := range e.active {
		out[k] = v
	}
	return out
}

func (e *Engine) RestoreActive(m map[string]alert.Alert) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range m {
		e.active[k] = v
	}
}

// Reset clears the in-memory active-alert set AND the dedupe history.
// Returns (active, dedupe) counts that were cleared.
func (e *Engine) Reset() (int, int) {
	e.mu.Lock()
	activeN := len(e.active)
	e.active = map[string]alert.Alert{}
	e.mu.Unlock()
	dedupeN := 0
	if e.opts.Dedupe != nil {
		dedupeN = e.opts.Dedupe.Reset()
	}
	return activeN, dedupeN
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
		case in := <-e.in:
			switch in.kind {
			case inputFire:
				e.handleFire(ctx, in.alert)
			case inputResolve:
				e.handleResolve(ctx, in.alert)
			case inputReconcile:
				e.handleReconcile(ctx, in.monitor, in.firing)
			}
		case <-tickC:
			e.flushDigest(ctx)
		}
	}
}

func (e *Engine) handleFire(ctx context.Context, a alert.Alert) {
	if !e.opts.Dedupe.Allow(a) {
		// even if suppressed, remember it is active so a future Resolve fires
		if e.opts.ResolutionEnabled {
			e.mu.Lock()
			e.active[a.Key()] = a
			e.mu.Unlock()
		}
		return
	}
	if e.opts.ResolutionEnabled {
		e.mu.Lock()
		e.active[a.Key()] = a
		e.mu.Unlock()
	}
	if e.opts.DigestEnabled && containsSev(e.opts.DigestSeverities, a.Severity) {
		e.mu.Lock()
		e.digestQ = append(e.digestQ, a)
		e.mu.Unlock()
		return
	}
	channels := e.opts.Router.Channels(a)
	if err := e.opts.Registry.Send(ctx, a, channels); err != nil {
		log.Printf("notifier send failed (monitor=%s severity=%s object=%s/%s): %v",
			a.Monitor, a.Severity.String(), a.Namespace, a.Object(), err)
	}
}

func (e *Engine) handleResolve(ctx context.Context, a alert.Alert) {
	if !e.opts.ResolutionEnabled {
		return
	}
	e.mu.Lock()
	prev, ok := e.active[a.Key()]
	if ok {
		delete(e.active, a.Key())
	}
	e.mu.Unlock()
	if !ok {
		// nothing was firing; don't send a spurious resolved
		return
	}
	resolved := mergeResolved(prev, a)
	channels := e.opts.Router.Channels(resolved)
	if err := e.opts.Registry.Send(ctx, resolved, channels); err != nil {
		log.Printf("notifier send failed (monitor=%s state=resolved object=%s/%s): %v",
			resolved.Monitor, resolved.Namespace, resolved.Object(), err)
	}
}

func (e *Engine) handleReconcile(ctx context.Context, monitor string, firing []alert.Alert) {
	firingKeys := make(map[string]struct{}, len(firing))
	for _, a := range firing {
		firingKeys[a.Key()] = struct{}{}
		e.handleFire(ctx, a)
	}
	if !e.opts.ResolutionEnabled {
		return
	}
	e.mu.Lock()
	var toResolve []alert.Alert
	for k, prev := range e.active {
		if prev.Monitor != monitor {
			continue
		}
		if _, ok := firingKeys[k]; ok {
			continue
		}
		toResolve = append(toResolve, prev)
		delete(e.active, k)
	}
	e.mu.Unlock()
	for _, prev := range toResolve {
		resolved := mergeResolved(prev, alert.Alert{})
		channels := e.opts.Router.Channels(resolved)
		if err := e.opts.Registry.Send(ctx, resolved, channels); err != nil {
			log.Printf("notifier send failed (monitor=%s state=resolved object=%s/%s): %v",
				resolved.Monitor, resolved.Namespace, resolved.Object(), err)
		}
	}
}

func mergeResolved(prev, hint alert.Alert) alert.Alert {
	out := prev
	out.State = alert.StateResolved
	out.Severity = alert.Info
	out.FiredAt = time.Now().UTC()
	if hint.Title != "" {
		out.Title = hint.Title
	} else {
		out.Title = "Resolved: " + prev.Title
	}
	if hint.Body != "" {
		out.Body = hint.Body
	} else {
		out.Body = "kpulse no longer detects the condition that triggered this alert."
	}
	return out
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
	if err := e.opts.Registry.Send(ctx, digest, channels); err != nil {
		log.Printf("notifier send failed (digest, %d alerts): %v", len(batch), err)
	}
}

func containsSev(list []alert.Severity, s alert.Severity) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
