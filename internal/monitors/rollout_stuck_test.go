package monitors

import (
	"sync"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type recordingSub struct {
	mu       sync.Mutex
	fired    []alert.Alert
	resolved []alert.Alert
}

func (r *recordingSub) Submit(a alert.Alert) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fired = append(r.fired, a)
}
func (r *recordingSub) Resolve(a alert.Alert) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolved = append(r.resolved, a)
}
func (r *recordingSub) Reconcile(_ string, firing []alert.Alert) {
	for _, a := range firing {
		r.Submit(a)
	}
}

func ssAt(ns, name string, replicas, ready int32, createdAgo time.Duration, updateRev, currentRev string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-createdAgo)},
		},
		Spec: appsv1.StatefulSetSpec{Replicas: &replicas},
		Status: appsv1.StatefulSetStatus{
			Replicas:        replicas,
			ReadyReplicas:   ready,
			UpdateRevision:  updateRev,
			CurrentRevision: currentRev,
		},
	}
}

// In v0.2.6 a redeploy of an old StatefulSet fired an immediate "stuck" +
// "resolved" pair because the check was `now - CreationTimestamp > threshold`.
// v0.2.7 ties the threshold to "how long has THIS rollout been in progress?".
func TestStatefulSetRedeployDoesNotFalseFire(t *testing.T) {
	cfg := &config.Config{
		Namespaces: config.NamespaceFilter{Include: []string{"*"}},
		Monitors:   config.Monitors{RolloutStuck: config.RolloutStuckMon{Enabled: true, Threshold: 15 * time.Minute}},
	}
	m := NewRolloutStuck(fake.NewSimpleClientset(), cfg)
	now := time.Now()
	m.now = func() time.Time { return now }
	sub := &recordingSub{}
	// StatefulSet is 2 hours old (much older than threshold) and a rollout has
	// JUST started (revisions differ, one pod not ready yet).
	s := ssAt("ns", "db", 3, 2, 2*time.Hour, "rev-2", "rev-1")
	// Direct call into the handler logic by simulating the SS informer event.
	threshold := 15 * time.Minute
	checkSS := buildSSChecker(m, sub, threshold)
	checkSS(s)

	if got := len(sub.fired); got != 0 {
		t.Fatalf("rollout that just started should not fire stuck; got %d alerts", got)
	}
	if got := len(sub.resolved); got != 0 {
		t.Fatalf("nothing was firing, should not have resolved; got %d", got)
	}

	// 16 minutes pass and the rollout still has not completed.
	m.now = func() time.Time { return now.Add(16 * time.Minute) }
	checkSS(s)
	if got := len(sub.fired); got != 1 {
		t.Fatalf("after threshold elapsed, expected 1 fire; got %d", got)
	}

	// Rollout completes. Revisions match, all pods ready.
	healthy := ssAt("ns", "db", 3, 3, 2*time.Hour, "rev-2", "rev-2")
	checkSS(healthy)
	if got := len(sub.resolved); got != 1 {
		t.Fatalf("expected 1 resolve once healthy; got %d", got)
	}
}

func TestStatefulSetClearedTimerOnHealthyBetween(t *testing.T) {
	cfg := &config.Config{
		Namespaces: config.NamespaceFilter{Include: []string{"*"}},
		Monitors:   config.Monitors{RolloutStuck: config.RolloutStuckMon{Enabled: true, Threshold: 15 * time.Minute}},
	}
	m := NewRolloutStuck(fake.NewSimpleClientset(), cfg)
	now := time.Now()
	m.now = func() time.Time { return now }
	sub := &recordingSub{}
	threshold := 15 * time.Minute
	checkSS := buildSSChecker(m, sub, threshold)

	// Rollout starts.
	checkSS(ssAt("ns", "db", 3, 2, 2*time.Hour, "rev-2", "rev-1"))
	// Becomes healthy 5 min later.
	m.now = func() time.Time { return now.Add(5 * time.Minute) }
	checkSS(ssAt("ns", "db", 3, 3, 2*time.Hour, "rev-2", "rev-2"))
	// New rollout 1h later.
	m.now = func() time.Time { return now.Add(1 * time.Hour) }
	checkSS(ssAt("ns", "db", 3, 2, 2*time.Hour, "rev-3", "rev-2"))
	if got := len(sub.fired); got != 0 {
		t.Fatalf("freshly-started 2nd rollout should not fire immediately; got %d", got)
	}

	// 16 min into the 2nd rollout it should fire.
	m.now = func() time.Time { return now.Add(1*time.Hour + 16*time.Minute) }
	checkSS(ssAt("ns", "db", 3, 2, 2*time.Hour, "rev-3", "rev-2"))
	if got := len(sub.fired); got != 1 {
		t.Fatalf("2nd rollout exceeded threshold; expected 1 fire, got %d", got)
	}
}

// buildSSChecker reproduces the closure inside RolloutStuck.Run for testing
// without spinning up informers. Keep in sync with the implementation.
func buildSSChecker(m *RolloutStuck, sub Submitter, threshold time.Duration) func(*appsv1.StatefulSet) {
	return func(s *appsv1.StatefulSet) {
		if !m.cfg.Namespaces.Allows(s.Namespace) {
			return
		}
		key := "StatefulSet|" + s.Namespace + "|" + s.Name
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
			})
			m.markFiring(key)
		} else if !inProgress && m.clearFiring(key) {
			sub.Resolve(alert.Alert{
				Monitor: m.Name(), Namespace: s.Namespace, ObjectKind: "StatefulSet", ObjectName: s.Name,
				Reason: "RolloutStuck",
			})
		}
	}
}
