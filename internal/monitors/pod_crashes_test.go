package monitors

import (
	"context"
	"testing"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type chanSub struct{ ch chan alert.Alert }

func (c *chanSub) Submit(a alert.Alert) {
	select {
	case c.ch <- a:
	default:
	}
}

func TestPodCrashesFiresOnOOMKilled(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cfg := &config.Config{
		Namespaces: config.NamespaceFilter{Include: []string{"*"}},
		Monitors:   config.Monitors{PodCrashes: config.PodCrashesMon{Enabled: true, Reasons: []string{"OOMKilled"}}},
	}
	m := NewPodCrashes(cs, cfg)
	sub := &chanSub{ch: make(chan alert.Alert, 4)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = m.Run(ctx, sub) }()

	time.Sleep(200 * time.Millisecond)

	_, err := cs.CoreV1().Pods("default").Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "c", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "OOMKilled"}},
		}}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case a := <-sub.ch:
		if a.Reason != "OOMKilled" || a.ObjectName != "p" {
			t.Errorf("got %+v", a)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no alert fired")
	}
}
