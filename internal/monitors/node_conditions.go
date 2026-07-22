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

type NodeConditions struct {
	cs   kubernetes.Interface
	cfg  *config.Config
	mu   sync.Mutex
	seen map[string]struct{} // key = node + "|" + conditionName
}

func NewNodeConditions(cs kubernetes.Interface, cfg *config.Config) *NodeConditions {
	return &NodeConditions{cs: cs, cfg: cfg, seen: map[string]struct{}{}}
}

func splitNodeCondKey(s string) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == '|' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}

func (m *NodeConditions) Name() string { return "node_conditions" }

// shouldAlert decides whether a firing condition is worth waking someone for.
//
// A node on its way out of the cluster is NotReady by design, and a node that just joined is
// NotReady until its kubelet settles. Both are routine capacity changes. Only a Ready state
// that persists past the grace window is an incident. Pressure conditions are unrelated to
// churn and are never delayed.
func (m *NodeConditions) shouldAlert(n *corev1.Node, condName string, now time.Time) bool {
	if condName != "NotReady" {
		return true
	}
	if n.DeletionTimestamp != nil {
		return false
	}
	for _, t := range n.Spec.Taints {
		if t.Key == "ToBeDeletedByClusterAutoscaler" || t.Key == "node.kubernetes.io/unschedulable" {
			return false
		}
	}
	grace := m.cfg.Monitors.NodeConditions.Grace
	if grace <= 0 {
		return true
	}
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return now.Sub(c.LastTransitionTime.Time) >= grace
		}
	}
	return true
}

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
		// Build the set of currently-firing condition names for this node.
		current := map[string]corev1.NodeCondition{}
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
			if fire {
				current[name] = cond
			}
		}
		// Fire each currently-firing.
		for name, cond := range current {
			if !m.shouldAlert(n, name, time.Now()) {
				continue
			}
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Critical,
				ObjectKind: "Node", ObjectName: n.Name,
				Reason: name,
				Title:  fmt.Sprintf("Node %s: %s", n.Name, name),
				Body:   fmt.Sprintf("Node condition %s reason=%s message=%s", name, cond.Reason, cond.Message),
			})
			m.mu.Lock()
			m.seen[n.Name+"|"+name] = struct{}{}
			m.mu.Unlock()
		}
		// Resolve any previously-firing condition for this node that is no longer present.
		m.mu.Lock()
		var toResolve []string
		for key := range m.seen {
			parts := splitNodeCondKey(key)
			if parts[0] != n.Name {
				continue
			}
			if _, still := current[parts[1]]; !still {
				toResolve = append(toResolve, key)
			}
		}
		for _, key := range toResolve {
			delete(m.seen, key)
		}
		m.mu.Unlock()
		for _, key := range toResolve {
			parts := splitNodeCondKey(key)
			sub.Resolve(alert.Alert{
				Monitor: m.Name(), ObjectKind: "Node", ObjectName: parts[0], Reason: parts[1],
				Title: fmt.Sprintf("Node %s: %s cleared", parts[0], parts[1]),
				Body:  "The condition is no longer reported by the kubelet.",
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
