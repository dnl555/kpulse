package monitors

import (
	"context"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodCrashes struct {
	cs  kubernetes.Interface
	cfg *config.Config
}

func NewPodCrashes(cs kubernetes.Interface, cfg *config.Config) *PodCrashes {
	return &PodCrashes{cs: cs, cfg: cfg}
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
		if _, hit := reasons[reason]; !hit {
			continue
		}
		sub.Submit(alert.Alert{
			Monitor: m.Name(), Severity: alert.Critical,
			Namespace: pod.Namespace, ObjectKind: "Pod", ObjectName: pod.Name,
			Reason: reason,
			Title:  reason + " on " + pod.Name + "/" + cs.Name,
			Body:   "Container " + cs.Name + " in pod " + pod.Namespace + "/" + pod.Name + " is in state " + reason,
		})
	}
}
