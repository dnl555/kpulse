package monitors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type HPAAtMax struct {
	cs    kubernetes.Interface
	cfg   *config.Config
	mu    sync.Mutex
	since map[string]time.Time
	now   func() time.Time
}

func NewHPAAtMax(cs kubernetes.Interface, cfg *config.Config) *HPAAtMax {
	return &HPAAtMax{cs: cs, cfg: cfg, since: map[string]time.Time{}, now: time.Now}
}

func (m *HPAAtMax) Name() string { return "hpa_at_max" }

func (m *HPAAtMax) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()
	handler := func(obj any) {
		h, ok := obj.(*autoscalingv2.HorizontalPodAutoscaler)
		if !ok {
			return
		}
		if !m.cfg.Namespaces.Allows(h.Namespace) {
			return
		}
		key := h.Namespace + "/" + h.Name
		m.mu.Lock()
		defer m.mu.Unlock()
		if h.Status.CurrentReplicas == h.Spec.MaxReplicas && h.Spec.MaxReplicas > 0 {
			if _, ok := m.since[key]; !ok {
				m.since[key] = m.now()
			}
			if m.now().Sub(m.since[key]) >= m.cfg.Monitors.HPAAtMax.Duration {
				sub.Submit(alert.Alert{
					Monitor: m.Name(), Severity: alert.Warning,
					Namespace: h.Namespace, ObjectKind: "HorizontalPodAutoscaler", ObjectName: h.Name,
					Reason: "AtMaxReplicas",
					Title:  fmt.Sprintf("HPA %s/%s pinned at maxReplicas=%d", h.Namespace, h.Name, h.Spec.MaxReplicas),
					Body:   fmt.Sprintf("currentReplicas=%d, since=%s", h.Status.CurrentReplicas, m.since[key].Format(time.RFC3339)),
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
