package monitors

import (
	"context"
	"fmt"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type NodeDisk struct {
	cs      kubernetes.Interface
	cfg     *config.Config
	fetcher summaryFetcher
}

func NewNodeDisk(cs kubernetes.Interface, cfg *config.Config) *NodeDisk {
	return &NodeDisk{cs: cs, cfg: cfg, fetcher: &restFetcher{cs}}
}

func (n *NodeDisk) Name() string { return "node_disk" }

func (n *NodeDisk) Run(ctx context.Context, sub Submitter) error {
	t := time.NewTicker(n.cfg.Monitors.NodeDisk.Interval)
	defer t.Stop()
	n.scan(ctx, sub)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			n.scan(ctx, sub)
		}
	}
}

func (n *NodeDisk) scan(ctx context.Context, sub Submitter) {
	nodes, err := n.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, node := range nodes.Items {
		s, err := n.fetcher.Fetch(ctx, node.Name)
		if err != nil {
			continue
		}
		n.checkFS(node.Name, "rootfs", s.Node.Fs, sub)
		n.checkFS(node.Name, "imagefs", s.Node.Runtime.ImageFs, sub)
	}
}

func (n *NodeDisk) checkFS(nodeName, fsKind string, fs nodeFs, sub Submitter) {
	if fs.CapacityBytes <= 0 {
		return
	}
	pct := float64(fs.UsedBytes) / float64(fs.CapacityBytes) * 100
	var sev alert.Severity
	switch {
	case pct >= n.cfg.Monitors.NodeDisk.CritAt:
		sev = alert.Critical
	case pct >= n.cfg.Monitors.NodeDisk.WarnAt:
		sev = alert.Warning
	default:
		return
	}
	sub.Submit(alert.Alert{
		Monitor: n.Name(), Severity: sev,
		ObjectKind: "Node", ObjectName: nodeName,
		Reason: "DiskHighUsage",
		Title:  fmt.Sprintf("Node %s %s at %.1f%%", nodeName, fsKind, pct),
		Body:   fmt.Sprintf("%s used %d / %d bytes (%.1f%%). Threshold warn=%.0f crit=%.0f.", fsKind, fs.UsedBytes, fs.CapacityBytes, pct, n.cfg.Monitors.NodeDisk.WarnAt, n.cfg.Monitors.NodeDisk.CritAt),
	})
}
