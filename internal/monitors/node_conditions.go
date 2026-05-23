package monitors

import (
	"context"
	"fmt"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type NodeConditions struct {
	cs  kubernetes.Interface
	cfg *config.Config
}

func NewNodeConditions(cs kubernetes.Interface, cfg *config.Config) *NodeConditions {
	return &NodeConditions{cs: cs, cfg: cfg}
}

func (m *NodeConditions) Name() string { return "node_conditions" }

func (m *NodeConditions) Run(ctx context.Context, sub Submitter) error {
	alertOn := map[string]struct{}{}
	for _, c := range m.cfg.Monitors.NodeConditions.AlertOn {
		alertOn[c] = struct{}{}
	}
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Core().V1().Nodes().Informer()
	handler := func(obj any) {
		n, ok := obj.(*corev1.Node)
		if !ok {
			return
		}
		for _, cond := range n.Status.Conditions {
			name := string(cond.Type)
			fire := false
			switch name {
			case "Ready":
				if _, ok := alertOn["NotReady"]; ok && cond.Status != corev1.ConditionTrue {
					fire = true
					name = "NotReady"
				}
			default:
				if _, ok := alertOn[name]; ok && cond.Status == corev1.ConditionTrue {
					fire = true
				}
			}
			if !fire {
				continue
			}
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Critical,
				ObjectKind: "Node", ObjectName: n.Name,
				Reason: name,
				Title:  fmt.Sprintf("Node %s: %s", n.Name, name),
				Body:   fmt.Sprintf("Node condition %s reason=%s message=%s", name, cond.Reason, cond.Message),
			})
		}
	}
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler,
		UpdateFunc: func(_, obj any) { handler(obj) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}
