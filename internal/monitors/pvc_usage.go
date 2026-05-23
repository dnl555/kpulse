package monitors

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type pvcRef struct{ Name, Namespace string }

type volumeStat struct {
	Name          string  `json:"name"`
	CapacityBytes int64   `json:"capacityBytes"`
	UsedBytes     int64   `json:"usedBytes"`
	PvcRef        *pvcRef `json:"pvcRef,omitempty"`
}

type nodeFs struct {
	CapacityBytes int64 `json:"capacityBytes"`
	UsedBytes     int64 `json:"usedBytes"`
}

type nodeRuntime struct {
	ImageFs nodeFs `json:"imageFs"`
}

type nodeStat struct {
	Fs      nodeFs      `json:"fs"`
	Runtime nodeRuntime `json:"runtime"`
}

type podStat struct {
	Volume []volumeStat `json:"volume"`
}

type kubeletSummary struct {
	Node nodeStat  `json:"node"`
	Pods []podStat `json:"pods"`
}

type summaryFetcher interface {
	Fetch(ctx context.Context, node string) (*kubeletSummary, error)
}

type restFetcher struct{ cs kubernetes.Interface }

func (r *restFetcher) Fetch(ctx context.Context, node string) (*kubeletSummary, error) {
	raw, err := r.cs.CoreV1().RESTClient().Get().
		AbsPath(fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", node)).
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var s kubeletSummary
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

type PVCUsage struct {
	cs      kubernetes.Interface
	cfg     *config.Config
	fetcher summaryFetcher
}

func NewPVCUsage(cs kubernetes.Interface, cfg *config.Config) *PVCUsage {
	return &PVCUsage{cs: cs, cfg: cfg, fetcher: &restFetcher{cs}}
}

func (p *PVCUsage) Name() string { return "pvc_usage" }

func (p *PVCUsage) Run(ctx context.Context, sub Submitter) error {
	t := time.NewTicker(p.cfg.Monitors.PVCUsage.Interval)
	defer t.Stop()
	p.scan(ctx, sub)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			p.scan(ctx, sub)
		}
	}
}

func (p *PVCUsage) scan(ctx context.Context, sub Submitter) {
	nodes, err := p.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	type usage struct{ used, cap int64 }
	pvc := map[pvcRef]usage{}
	for _, n := range nodes.Items {
		s, err := p.fetcher.Fetch(ctx, n.Name)
		if err != nil {
			continue
		}
		for _, pod := range s.Pods {
			for _, v := range pod.Volume {
				if v.PvcRef == nil || v.CapacityBytes <= 0 {
					continue
				}
				pvc[*v.PvcRef] = usage{used: v.UsedBytes, cap: v.CapacityBytes}
			}
		}
	}
	for ref, u := range pvc {
		if !p.cfg.Namespaces.Allows(ref.Namespace) {
			continue
		}
		pct := float64(u.used) / float64(u.cap) * 100
		var sev alert.Severity
		switch {
		case pct >= p.cfg.Monitors.PVCUsage.CritAt:
			sev = alert.Critical
		case pct >= p.cfg.Monitors.PVCUsage.WarnAt:
			sev = alert.Warning
		default:
			continue
		}
		sub.Submit(alert.Alert{
			Monitor: p.Name(), Severity: sev,
			Namespace: ref.Namespace, ObjectKind: "PersistentVolumeClaim", ObjectName: ref.Name,
			Reason: "PVCHighUsage",
			Title:  fmt.Sprintf("PVC %s/%s at %.1f%%", ref.Namespace, ref.Name, pct),
			Body:   fmt.Sprintf("Used %d / %d bytes (%.1f%%). Threshold warn=%.0f crit=%.0f.", u.used, u.cap, pct, p.cfg.Monitors.PVCUsage.WarnAt, p.cfg.Monitors.PVCUsage.CritAt),
		})
	}
}
