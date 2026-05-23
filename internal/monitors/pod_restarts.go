package monitors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type restartSample struct {
	at    time.Time
	count int32
}

type PodRestarts struct {
	cs   kubernetes.Interface
	cfg  *config.Config
	mu   sync.Mutex
	hist map[string][]restartSample
	now  func() time.Time
}

func NewPodRestarts(cs kubernetes.Interface, cfg *config.Config) *PodRestarts {
	return &PodRestarts{cs: cs, cfg: cfg, hist: map[string][]restartSample{}, now: time.Now}
}

func (m *PodRestarts) Name() string { return "pod_restarts" }

func (m *PodRestarts) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Core().V1().Pods().Informer()
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, obj any) { m.handle(obj, sub) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}

func (m *PodRestarts) handle(obj any, sub Submitter) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	if !m.cfg.Namespaces.Allows(pod.Namespace) {
		return
	}
	window := m.cfg.Monitors.PodRestarts.Window
	threshold := m.cfg.Monitors.PodRestarts.Threshold
	if window <= 0 || threshold <= 0 {
		return
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cs := range pod.Status.ContainerStatuses {
		key := string(pod.UID) + "/" + cs.Name
		samples := append(m.hist[key], restartSample{at: now, count: cs.RestartCount})
		cutoff := now.Add(-window)
		trimmed := samples[:0]
		for _, s := range samples {
			if s.at.After(cutoff) || s.at.Equal(cutoff) {
				trimmed = append(trimmed, s)
			}
		}
		m.hist[key] = trimmed
		if len(trimmed) < 2 {
			continue
		}
		delta := trimmed[len(trimmed)-1].count - trimmed[0].count
		if int(delta) >= threshold {
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Warning,
				Namespace: pod.Namespace, ObjectKind: "Pod", ObjectName: pod.Name,
				Reason: "RestartStorm",
				Title:  fmt.Sprintf("%s/%s restarted %d times in %s", pod.Name, cs.Name, delta, window),
				Body:   fmt.Sprintf("Container %s in pod %s/%s restarted %d times within %s.", cs.Name, pod.Namespace, pod.Name, delta, window),
			})
		}
	}
}
