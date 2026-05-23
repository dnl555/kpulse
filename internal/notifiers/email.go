package notifiers

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/dnl555/kpulse/internal/alert"
)

type smtpAuth = smtp.Auth

type smtpSender interface {
	SendMail(addr string, a smtpAuth, from string, to []string, msg []byte) error
}

type realSMTP struct{}

func (realSMTP) SendMail(addr string, a smtpAuth, from string, to []string, msg []byte) error {
	return smtp.SendMail(addr, a, from, to, msg)
}

type Email struct {
	host     string
	port     int
	username string
	password string
	from     string
	to       []string
	sender   smtpSender
}

func NewEmail(host string, port int, user, pass, from string, to []string) *Email {
	return &Email{host: host, port: port, username: user, password: pass, from: from, to: to, sender: realSMTP{}}
}

func (e *Email) Name() string { return "email" }

func (e *Email) Send(_ context.Context, a alert.Alert) error {
	subj := fmt.Sprintf("[kpulse][%s][%s] %s", a.Cluster, a.Severity.String(), a.Title)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", e.from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(e.to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subj)
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	fmt.Fprintf(&b, "Cluster:   %s\nNamespace: %s\nObject:    %s\nReason:    %s\nSeverity:  %s\nFiredAt:   %s\n\n",
		a.Cluster, a.Namespace, a.Object(), a.Reason, a.Severity.String(), a.FiredAt.Format("2006-01-02 15:04:05 MST"))
	b.WriteString(a.Body)

	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.host)
	}
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	return e.sender.SendMail(addr, auth, e.from, e.to, []byte(b.String()))
}
