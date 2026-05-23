package monitors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type DaemonSetUnscheduled struct {
	cs    kubernetes.Interface
	cfg   *config.Config
	mu    sync.Mutex
	since map[string]time.Time
	now   func() time.Time
}

func NewDaemonSetUnscheduled(cs kubernetes.Interface, cfg *config.Config) *DaemonSetUnscheduled {
	return &DaemonSetUnscheduled{cs: cs, cfg: cfg, since: map[string]time.Time{}, now: time.Now}
}

func (m *DaemonSetUnscheduled) Name() string { return "daemonset_unscheduled" }

func (m *DaemonSetUnscheduled) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Apps().V1().DaemonSets().Informer()
	handler := func(obj any) {
		d, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			return
		}
		if !m.cfg.Namespaces.Allows(d.Namespace) {
			return
		}
		key := d.Namespace + "/" + d.Name
		m.mu.Lock()
		defer m.mu.Unlock()
		if d.Status.DesiredNumberScheduled != d.Status.NumberReady {
			if _, ok := m.since[key]; !ok {
				m.since[key] = m.now()
			}
			if m.now().Sub(m.since[key]) >= m.cfg.Monitors.DaemonSetUnscheduled.Threshold {
				sub.Submit(alert.Alert{
					Monitor: m.Name(), Severity: alert.Warning,
					Namespace: d.Namespace, ObjectKind: "DaemonSet", ObjectName: d.Name,
					Reason: "Unscheduled",
					Title:  fmt.Sprintf("DaemonSet %s/%s: %d/%d ready", d.Namespace, d.Name, d.Status.NumberReady, d.Status.DesiredNumberScheduled),
					Body:   fmt.Sprintf("Desired %d, ready %d (since %s).", d.Status.DesiredNumberScheduled, d.Status.NumberReady, m.since[key].Format(time.RFC3339)),
				})
			}
		} else {
			delete(m.since, key)
		}
	}
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: handler, UpdateFunc: func(_, obj any) { handler(obj) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}
