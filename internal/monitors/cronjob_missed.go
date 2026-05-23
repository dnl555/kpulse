package monitors

import (
	"context"
	"fmt"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CronJobMissed struct {
	cs  kubernetes.Interface
	cfg *config.Config
	now func() time.Time
}

func NewCronJobMissed(cs kubernetes.Interface, cfg *config.Config) *CronJobMissed {
	return &CronJobMissed{cs: cs, cfg: cfg, now: time.Now}
}

func (m *CronJobMissed) Name() string { return "cronjob_missed" }

func (m *CronJobMissed) Run(ctx context.Context, sub Submitter) error {
	t := time.NewTicker(1 * time.Minute)
	defer t.Stop()
	m.scan(ctx, sub)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			m.scan(ctx, sub)
		}
	}
}

func (m *CronJobMissed) scan(ctx context.Context, sub Submitter) {
	list, err := m.cs.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	miss := m.cfg.Monitors.CronJobMissed.MissThreshold
	if miss <= 0 {
		miss = 2
	}
	for _, cj := range list.Items {
		if !m.cfg.Namespaces.Allows(cj.Namespace) {
			continue
		}
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			continue
		}
		sched, err := parser.Parse(cj.Spec.Schedule)
		if err != nil {
			continue
		}
		lastRef := cj.CreationTimestamp.Time
		if cj.Status.LastScheduleTime != nil {
			lastRef = cj.Status.LastScheduleTime.Time
		}
		now := m.now()
		expected := sched.Next(lastRef)
		missed := 0
		for !expected.After(now) {
			missed++
			expected = sched.Next(expected)
			if missed > miss+10 {
				break
			}
		}
		if missed >= miss {
			sub.Submit(alert.Alert{
				Monitor: m.Name(), Severity: alert.Warning,
				Namespace: cj.Namespace, ObjectKind: "CronJob", ObjectName: cj.Name,
				Reason: "MissedSchedules",
				Title:  fmt.Sprintf("CronJob %s/%s missed %d schedules", cj.Namespace, cj.Name, missed),
				Body:   fmt.Sprintf("Last scheduled at %s; expected %d more runs by %s.", lastRef.Format(time.RFC3339), missed, now.Format(time.RFC3339)),
			})
		}
	}
}
