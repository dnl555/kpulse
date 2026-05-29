package state

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSaveAndLoad(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "kpulse-state", Namespace: "kpulse"},
	})
	s := New(cs, "kpulse", "kpulse-state")
	now := time.Now().UTC().Truncate(time.Second)
	want := map[string]time.Time{"key1": now, "key2": now.Add(time.Minute)}

	if err := s.Save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["key1"].Equal(want["key1"]) {
		t.Errorf("got %+v want %+v", got, want)
	}
}

func TestSaveCreatesConfigMapIfMissing(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := New(cs, "kpulse", "kpulse-state")
	if err := s.Save(context.Background(), map[string]time.Time{"k": time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := cs.CoreV1().ConfigMaps("kpulse").Get(context.Background(), "kpulse-state", metav1.GetOptions{}); err != nil {
		t.Fatalf("CM not created: %v", err)
	}
}

func TestClearEmptiesConfigMapWithoutDeleting(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := New(cs, "kpulse", "kpulse-state")
	now := time.Now().UTC()
	if err := s.Save(context.Background(), map[string]time.Time{"k1": now, "k2": now}); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	cm, err := cs.CoreV1().ConfigMaps("kpulse").Get(context.Background(), "kpulse-state", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("CM should still exist (Clear must not delete): %v", err)
	}
	if got := cm.Data["dedupe.json"]; got != "{}" {
		t.Errorf("dedupe.json after Clear = %q, want %q", got, "{}")
	}
	got, err := s.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Load after Clear returned %d entries, want 0", len(got))
	}
}

func TestClearMissingCMIsNoop(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := New(cs, "kpulse", "kpulse-state")
	if err := s.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
}
