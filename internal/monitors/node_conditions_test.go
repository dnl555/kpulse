package monitors

import (
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func node(name string, ready corev1.ConditionStatus, transition time.Time) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(transition)},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: ready, LastTransitionTime: metav1.NewTime(transition)},
		}},
	}
}

func cfgWithGrace(g time.Duration) *config.Config {
	c := &config.Config{}
	c.Monitors.NodeConditions.Enabled = true
	c.Monitors.NodeConditions.AlertOn = []string{"NotReady"}
	c.Monitors.NodeConditions.Grace = g
	return c
}

// A node the autoscaler is removing is NotReady on its way out. That is routine capacity
// churn, not an incident, and paging on it trains people to ignore node alerts.
func TestNodeBeingDeletedIsNotAlerted(t *testing.T) {
	m := NewNodeConditions(nil, cfgWithGrace(5*time.Minute))
	n := node("scaling-in", corev1.ConditionFalse, time.Now().Add(-time.Hour))
	now := metav1.NewTime(time.Now())
	n.DeletionTimestamp = &now
	if m.shouldAlert(n, "NotReady", time.Now()) {
		t.Fatal("a node being deleted must not raise a node alert")
	}
}

// A node the autoscaler just added is NotReady until the kubelet settles.
func TestFreshlyNotReadyIsWithheldUntilGraceElapses(t *testing.T) {
	m := NewNodeConditions(nil, cfgWithGrace(5*time.Minute))
	n := node("scaling-out", corev1.ConditionFalse, time.Now().Add(-30*time.Second))
	if m.shouldAlert(n, "NotReady", time.Now()) {
		t.Fatal("a node NotReady for 30s must not alert under a 5m grace")
	}
}

// A node that stays down is a real incident and must still page.
func TestSustainedNotReadyStillAlerts(t *testing.T) {
	m := NewNodeConditions(nil, cfgWithGrace(5*time.Minute))
	n := node("really-broken", corev1.ConditionFalse, time.Now().Add(-20*time.Minute))
	if !m.shouldAlert(n, "NotReady", time.Now()) {
		t.Fatal("a node NotReady for 20m must alert")
	}
}

// Pressure conditions are not churn-related and should not be delayed.
func TestOtherConditionsAreNotDelayed(t *testing.T) {
	m := NewNodeConditions(nil, cfgWithGrace(5*time.Minute))
	n := node("under-pressure", corev1.ConditionTrue, time.Now().Add(-10*time.Second))
	if !m.shouldAlert(n, "DiskPressure", time.Now()) {
		t.Fatal("DiskPressure must alert immediately")
	}
}
