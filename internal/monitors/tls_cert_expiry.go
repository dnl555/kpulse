package monitors

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type TLSCertExpiry struct {
	cs  kubernetes.Interface
	cfg *config.Config
	now func() time.Time
}

func NewTLSCertExpiry(cs kubernetes.Interface, cfg *config.Config) *TLSCertExpiry {
	return &TLSCertExpiry{cs: cs, cfg: cfg, now: time.Now}
}

func (t *TLSCertExpiry) Name() string { return "tls_cert_expiry" }

func (t *TLSCertExpiry) Run(ctx context.Context, sub Submitter) error {
	tick := time.NewTicker(t.cfg.Monitors.TLSCertExpiry.Interval)
	defer tick.Stop()
	t.scan(ctx, sub)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			t.scan(ctx, sub)
		}
	}
}

func (t *TLSCertExpiry) scan(ctx context.Context, sub Submitter) {
	secrets, err := t.cs.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		FieldSelector: "type=" + string(corev1.SecretTypeTLS),
	})
	if err != nil {
		return
	}
	var firing []alert.Alert
	for _, s := range secrets.Items {
		if !t.cfg.Namespaces.Allows(s.Namespace) {
			continue
		}
		crt := s.Data["tls.crt"]
		if len(crt) == 0 {
			continue
		}
		block, _ := pem.Decode(crt)
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		days := int(cert.NotAfter.Sub(t.now()).Hours() / 24)
		var sev alert.Severity
		switch {
		case days <= t.cfg.Monitors.TLSCertExpiry.CritDays:
			sev = alert.Critical
		case days <= t.cfg.Monitors.TLSCertExpiry.WarnDays:
			sev = alert.Warning
		default:
			continue
		}
		firing = append(firing, alert.Alert{
			Monitor: t.Name(), Severity: sev,
			Namespace: s.Namespace, ObjectKind: "Secret", ObjectName: s.Name,
			Reason: "CertExpiringSoon",
			Title:  fmt.Sprintf("Cert in %s/%s expires in %d days", s.Namespace, s.Name, days),
			Body:   fmt.Sprintf("CN=%s NotAfter=%s", cert.Subject.CommonName, cert.NotAfter.Format(time.RFC3339)),
		})
	}
	sub.Reconcile(t.Name(), firing)
}
