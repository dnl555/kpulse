package monitors

import (
	"context"
	"sync"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodCrashes struct {
	cs   kubernetes.Interface
	cfg  *config.Config
	mu   sync.Mutex
	seen map[string]firingState // key = podUID|container, value = (ns, podName, reason)
}

type firingState struct {
	ns, podName, container, reason string
}

func NewPodCrashes(cs kubernetes.Interface, cfg *config.Config) *PodCrashes {
	return &PodCrashes{cs: cs, cfg: cfg, seen: map[string]firingState{}}
}

func (m *PodCrashes) Name() string { return "pod_crashes" }

func (m *PodCrashes) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Core().V1().Pods().Informer()
	reasons := map[string]struct{}{}
	for _, r := range m.cfg.Monitors.PodCrashes.Reasons {
		reasons[r] = struct{}{}
	}
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { m.handle(obj, sub, reasons) },
		UpdateFunc: func(_, obj any) { m.handle(obj, sub, reasons) },
		DeleteFunc: func(obj any) { m.handleDelete(obj, sub) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}

func (m *PodCrashes) handle(obj any, sub Submitter, reasons map[string]struct{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	if !m.cfg.Namespaces.Allows(pod.Namespace) {
		return
	}
	for _, cs := range pod.Status.ContainerStatuses {
		var reason string
		switch {
		case cs.State.Waiting != nil:
			reason = cs.State.Waiting.Reason
		case cs.State.Terminated != nil:
			reason = cs.State.Terminated.Reason
		}
		key := string(pod.UID) + "|" + cs.Name
		if _, hit := reasons[reason]; hit {
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Critical,
				Namespace: pod.Namespace, ObjectKind: "Pod", ObjectName: pod.Name,
				Reason: reason,
				Title:  reason + " on " + pod.Name + "/" + cs.Name,
				Body:   "Container " + cs.Name + " in pod " + pod.Namespace + "/" + pod.Name + " is in state " + reason,
			})
			m.mu.Lock()
			m.seen[key] = firingState{ns: pod.Namespace, podName: pod.Name, container: cs.Name, reason: reason}
			m.mu.Unlock()
			continue
		}
		// Container is healthy (Running, or Waiting/Terminated with a non-tracked reason).
		// If we had previously fired, resolve.
		if cs.State.Running != nil {
			m.mu.Lock()
			prev, was := m.seen[key]
			if was {
				delete(m.seen, key)
			}
			m.mu.Unlock()
			if was {
				sub.Resolve(alert.Alert{
					Monitor:   m.Name(),
					Namespace: prev.ns, ObjectKind: "Pod", ObjectName: prev.podName,
					Reason: prev.reason,
					Title:  prev.podName + "/" + prev.container + " is back to Running",
					Body:   "Container " + prev.container + " in pod " + prev.ns + "/" + prev.podName + " is now Running.",
				})
			}
		}
	}
}

func (m *PodCrashes) handleDelete(obj any, sub Submitter) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	prefix := string(pod.UID) + "|"
	m.mu.Lock()
	var resolved []firingState
	for k, v := range m.seen {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			resolved = append(resolved, v)
			delete(m.seen, k)
		}
	}
	m.mu.Unlock()
	for _, prev := range resolved {
		sub.Resolve(alert.Alert{
			Monitor:   m.Name(),
			Namespace: prev.ns, ObjectKind: "Pod", ObjectName: prev.podName,
			Reason: prev.reason,
			Title:  prev.podName + "/" + prev.container + " deleted",
			Body:   "Pod " + prev.ns + "/" + prev.podName + " no longer exists; clearing the alert.",
		})
	}
}
