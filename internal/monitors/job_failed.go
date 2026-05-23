package monitors

import (
	"context"
	"fmt"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type JobFailed struct {
	cs  kubernetes.Interface
	cfg *config.Config
}

func NewJobFailed(cs kubernetes.Interface, cfg *config.Config) *JobFailed {
	return &JobFailed{cs: cs, cfg: cfg}
}

func (m *JobFailed) Name() string { return "job_failed" }

func (m *JobFailed) Run(ctx context.Context, sub Submitter) error {
	factory := informers.NewSharedInformerFactory(m.cs, 0)
	inf := factory.Batch().V1().Jobs().Informer()
	handler := func(obj any) {
		j, ok := obj.(*batchv1.Job)
		if !ok {
			return
		}
		if !m.cfg.Namespaces.Allows(j.Namespace) {
			return
		}
		for _, c := range j.Status.Conditions {
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
				sub.Submit(alert.Alert{
					Monitor: m.Name(), Severity: alert.Warning,
					Namespace: j.Namespace, ObjectKind: "Job", ObjectName: j.Name,
					Reason: c.Reason,
					Title:  fmt.Sprintf("Job %s/%s failed", j.Namespace, j.Name),
					Body:   c.Message,
				})
			}
		}
	}
	_, _ = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: handler, UpdateFunc: func(_, obj any) { handler(obj) },
	})
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
	return nil
}
