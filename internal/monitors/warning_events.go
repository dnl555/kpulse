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

type WarningEvents struct {
	cs  kubernetes.Interface
	cfg *config.Config
}

func NewWarningEvents(cs kubernetes.Interface, cfg *config.Config) *WarningEvents {
	return &WarningEvents{cs: cs, cfg: cfg}
}

func (m *WarningEvents) Name() string { return "warning_events" }

func (m *WarningEvents) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Core().V1().Events().Informer()
	ignore := map[string]struct{}{}
	for _, r := range m.cfg.Monitors.WarningEvents.ReasonsIgnore {
		ignore[r] = struct{}{}
	}
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			ev, ok := obj.(*corev1.Event)
			if !ok || ev.Type != "Warning" {
				return
			}
			if !m.cfg.Namespaces.Allows(ev.Namespace) {
				return
			}
			if _, ig := ignore[ev.Reason]; ig {
				return
			}
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Info,
				Namespace: ev.Namespace, ObjectKind: ev.InvolvedObject.Kind, ObjectName: ev.InvolvedObject.Name,
				Reason: ev.Reason, Title: ev.Reason + " on " + ev.InvolvedObject.Name, Body: ev.Message,
			})
		},
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}
