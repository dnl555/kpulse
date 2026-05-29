package monitors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type RolloutStuck struct {
	cs    kubernetes.Interface
	cfg   *config.Config
	now   func() time.Time
	mu    sync.Mutex
	seen  map[string]struct{}  // fired alerts: key -> present
	since map[string]time.Time // key -> first-time-we-saw-this-rollout-in-progress
}

func NewRolloutStuck(cs kubernetes.Interface, cfg *config.Config) *RolloutStuck {
	return &RolloutStuck{
		cs: cs, cfg: cfg, now: time.Now,
		seen:  map[string]struct{}{},
		since: map[string]time.Time{},
	}
}

func (m *RolloutStuck) markFiring(k string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[k] = struct{}{}
}

func (m *RolloutStuck) clearFiring(k string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.seen[k]
	if ok {
		delete(m.seen, k)
	}
	delete(m.since, k)
	return ok
}

// rolloutStartedAt records the first time we observed an in-progress rollout
// for a given key. If we already had a timestamp, return it; otherwise stamp
// "now" and return that. Caller decides whether enough time has elapsed.
func (m *RolloutStuck) rolloutStartedAt(k string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.since[k]; ok {
		return t
	}
	t := m.now()
	m.since[k] = t
	return t
}

// rolloutHealthy clears the "since" stamp for a key. Used when the workload
// reports a healthy state, so the next time it gets updated we restart the
// stuck-detection timer from zero.
func (m *RolloutStuck) rolloutHealthy(k string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.since, k)
}

func (m *RolloutStuck) Name() string { return "rollout_stuck" }

func (m *RolloutStuck) Run(ctx context.Context, sub Submitter) error {
	threshold := m.cfg.Monitors.RolloutStuck.Threshold
	if threshold <= 0 {
		threshold = 15 * time.Minute
	}
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	depInf := factory.Apps().V1().Deployments().Informer()
	ssInf := factory.Apps().V1().StatefulSets().Informer()

	checkDep := func(obj any) {
		d, ok := obj.(*appsv1.Deployment)
		if !ok {
			return
		}
		if !m.cfg.Namespaces.Allows(d.Namespace) {
			return
		}
		key := "Deployment|" + d.Namespace + "|" + d.Name
		stuck := false
		var stuckCond appsv1.DeploymentCondition
		for _, c := range d.Status.Conditions {
			if c.Type != appsv1.DeploymentProgressing {
				continue
			}
			if c.Status == corev1.ConditionTrue {
				continue
			}
			if m.now().Sub(c.LastUpdateTime.Time) < threshold {
				continue
			}
			stuck = true
			stuckCond = c
			break
		}
		if stuck {
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Warning,
				Namespace: d.Namespace, ObjectKind: "Deployment", ObjectName: d.Name,
				Reason: "RolloutStuck",
				Title:  fmt.Sprintf("Deployment %s/%s rollout stuck", d.Namespace, d.Name),
				Body:   fmt.Sprintf("Progressing condition %s reason=%s message=%s", stuckCond.Status, stuckCond.Reason, stuckCond.Message),
			})
			m.markFiring(key)
		} else if m.clearFiring(key) {
			sub.Resolve(alert.Alert{
				Monitor: m.Name(), Namespace: d.Namespace, ObjectKind: "Deployment", ObjectName: d.Name,
				Reason: "RolloutStuck",
				Title:  fmt.Sprintf("Deployment %s/%s rollout completed", d.Namespace, d.Name),
				Body:   "The rollout has progressed past the stuck condition.",
			})
		}
	}
	checkSS := func(obj any) {
		s, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return
		}
		if !m.cfg.Namespaces.Allows(s.Namespace) {
			return
		}
		key := "StatefulSet|" + s.Namespace + "|" + s.Name
		// In-progress rollout: not all replicas are Ready, or revisions don't match.
		inProgress := s.Status.ReadyReplicas < s.Status.Replicas ||
			(s.Status.UpdateRevision != "" && s.Status.UpdateRevision != s.Status.CurrentRevision)
		stuck := false
		if inProgress {
			startedAt := m.rolloutStartedAt(key)
			stuck = m.now().Sub(startedAt) > threshold
		} else {
			m.rolloutHealthy(key)
		}
		if stuck {
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Warning,
				Namespace: s.Namespace, ObjectKind: "StatefulSet", ObjectName: s.Name,
				Reason: "RolloutStuck",
				Title:  fmt.Sprintf("StatefulSet %s/%s rollout stuck", s.Namespace, s.Name),
				Body:   fmt.Sprintf("Ready %d of %d for > %s", s.Status.ReadyReplicas, s.Status.Replicas, threshold),
			})
			m.markFiring(key)
		} else if !inProgress && m.clearFiring(key) {
			sub.Resolve(alert.Alert{
				Monitor: m.Name(), Namespace: s.Namespace, ObjectKind: "StatefulSet", ObjectName: s.Name,
				Reason: "RolloutStuck",
				Title:  fmt.Sprintf("StatefulSet %s/%s rollout completed", s.Namespace, s.Name),
				Body:   fmt.Sprintf("Ready %d of %d.", s.Status.ReadyReplicas, s.Status.Replicas),
			})
		}
	}

	_, _ = depInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: checkDep, UpdateFunc: func(_, obj any) { checkDep(obj) },
	})
	_, _ = ssInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: checkSS, UpdateFunc: func(_, obj any) { checkSS(obj) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}
