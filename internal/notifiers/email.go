package notifiers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

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
	from     string // accepted forms: "addr@host" OR `"Display Name" <addr@host>`
	replyTo  string // optional Reply-To header (same accepted forms as from)
	to       []string
	sender   smtpSender
	now      func() time.Time
}

func NewEmail(host string, port int, user, pass, from string, to []string) *Email {
	return &Email{host: host, port: port, username: user, password: pass, from: from, to: to, sender: realSMTP{}, now: time.Now}
}

// WithReplyTo sets an optional Reply-To header so that recipients hitting
// "reply" land in a different inbox than the (often unmonitored) From address.
// Returns the receiver for chained construction.
func (e *Email) WithReplyTo(replyTo string) *Email {
	e.replyTo = replyTo
	return e
}

// envelopeAddr extracts the bare address from a string that may be either
// "addr@host" or a full RFC 5322 mailbox like `"Display" <addr@host>`. The
// SMTP MAIL FROM command rejects display names, so we always strip them
// before passing to smtp.SendMail. The original (possibly-named) string is
// still used verbatim in the From: header.
func envelopeAddr(raw string) string {
	if a, err := mail.ParseAddress(raw); err == nil {
		return a.Address
	}
	return raw
}

func (e *Email) Name() string { return "email" }

const subjectMaxLen = 70

func (e *Email) Send(_ context.Context, a alert.Alert) error {
	msg, err := e.compose(a)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.host)
	}
	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	return e.sender.SendMail(addr, auth, envelopeAddr(e.from), e.to, msg)
}

func (e *Email) compose(a alert.Alert) ([]byte, error) {
	a.EnsureFiredAt()
	now := e.now()

	icon := severityIcon(a.Severity)
	if a.State == alert.StateResolved {
		icon = "[OK]"
	}
	subj := truncateSubject(fmt.Sprintf("[%s] %s %s", a.Cluster, icon, a.Title))
	if a.State == alert.StateResolved {
		subj = truncateSubject(fmt.Sprintf("[%s] [RESOLVED] %s", a.Cluster, a.Title))
	}
	msgID := fmt.Sprintf("<%s-%d@kpulse.local>", a.Key(), now.UnixNano())
	listID := fmt.Sprintf("kpulse-%s <kpulse.%s.local>", safeToken(a.Cluster), safeToken(a.Cluster))

	var out bytes.Buffer

	// Top-level headers go on the outer message. If we have attachments, the
	// outer Content-Type is multipart/mixed wrapping a multipart/alternative.
	// Otherwise the outer is multipart/alternative directly.
	hasAttachments := len(a.Attachments) > 0

	writeHeader := func(k, v string) { fmt.Fprintf(&out, "%s: %s\r\n", k, v) }
	writeHeader("From", e.from)
	if e.replyTo != "" {
		writeHeader("Reply-To", e.replyTo)
	}
	writeHeader("To", strings.Join(e.to, ", "))
	writeHeader("Subject", subj)
	writeHeader("Date", now.UTC().Format(time.RFC1123Z))
	writeHeader("Message-ID", msgID)
	writeHeader("List-Id", listID)
	writeHeader("Auto-Submitted", "auto-generated")
	writeHeader("X-Kpulse-Cluster", a.Cluster)
	writeHeader("X-Kpulse-Monitor", a.Monitor)
	writeHeader("X-Kpulse-Severity", a.Severity.String())
	writeHeader("X-Kpulse-State", a.State.String())
	writeHeader("MIME-Version", "1.0")

	if hasAttachments {
		mixed := multipart.NewWriter(&out)
		writeHeader("Content-Type", `multipart/mixed; boundary="`+mixed.Boundary()+`"`)
		out.WriteString("\r\n")

		// alternative part nested inside mixed
		altHeader := textproto.MIMEHeader{}
		altBuf := &bytes.Buffer{}
		alt := multipart.NewWriter(altBuf)
		altHeader.Set("Content-Type", `multipart/alternative; boundary="`+alt.Boundary()+`"`)
		if err := writeAlternative(alt, a); err != nil {
			return nil, err
		}
		_ = alt.Close()
		altPart, err := mixed.CreatePart(altHeader)
		if err != nil {
			return nil, err
		}
		if _, err := altPart.Write(altBuf.Bytes()); err != nil {
			return nil, err
		}

		for _, att := range a.Attachments {
			h := textproto.MIMEHeader{}
			ct := att.ContentType
			if ct == "" {
				ct = "application/octet-stream"
			}
			h.Set("Content-Type", ct)
			h.Set("Content-Transfer-Encoding", "base64")
			h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(att.Name)))
			p, err := mixed.CreatePart(h)
			if err != nil {
				return nil, err
			}
			enc := base64.StdEncoding.EncodeToString(att.Body)
			for i := 0; i < len(enc); i += 76 {
				end := i + 76
				if end > len(enc) {
					end = len(enc)
				}
				if _, err := p.Write([]byte(enc[i:end] + "\r\n")); err != nil {
					return nil, err
				}
			}
		}
		_ = mixed.Close()
		return out.Bytes(), nil
	}

	// No attachments: outer is multipart/alternative.
	alt := multipart.NewWriter(&out)
	writeHeader("Content-Type", `multipart/alternative; boundary="`+alt.Boundary()+`"`)
	out.WriteString("\r\n")
	if err := writeAlternative(alt, a); err != nil {
		return nil, err
	}
	_ = alt.Close()
	return out.Bytes(), nil
}

func writeAlternative(alt *multipart.Writer, a alert.Alert) error {
	plainHdr := textproto.MIMEHeader{}
	plainHdr.Set("Content-Type", "text/plain; charset=UTF-8")
	plainHdr.Set("Content-Transfer-Encoding", "8bit")
	p, err := alt.CreatePart(plainHdr)
	if err != nil {
		return err
	}
	if _, err := p.Write([]byte(plainBody(a))); err != nil {
		return err
	}

	htmlHdr := textproto.MIMEHeader{}
	htmlHdr.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHdr.Set("Content-Transfer-Encoding", "8bit")
	h, err := alt.CreatePart(htmlHdr)
	if err != nil {
		return err
	}
	_, err = h.Write([]byte(htmlBody(a)))
	return err
}

func plainBody(a alert.Alert) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", a.Title)
	fmt.Fprintf(&b, "Cluster:   %s\nNamespace: %s\nObject:    %s\nReason:    %s\nSeverity:  %s\nMonitor:   %s\nFiredAt:   %s\n\n",
		a.Cluster, a.Namespace, a.Object(), a.Reason, a.Severity.String(), a.Monitor,
		a.FiredAt.Format("2006-01-02 15:04:05 MST"))
	b.WriteString(a.Body)
	b.WriteString("\n\n---\nSent by kpulse  https://kpulse.io\n")
	return b.String()
}

func htmlBody(a alert.Alert) string {
	color := severityColor(a.Severity)
	icon := severityIcon(a.Severity)
	bannerLabel := strings.ToUpper(a.Severity.String())
	if a.State == alert.StateResolved {
		color = "#10b981"
		icon = "[OK]"
		bannerLabel = "RESOLVED"
	}
	esc := html.EscapeString

	row := func(k, v string) string {
		if v == "" {
			v = "-"
		}
		return fmt.Sprintf(
			`<tr><td style="padding:6px 12px;color:#6b7280;font:13px/1.4 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;white-space:nowrap;">%s</td>`+
				`<td style="padding:6px 12px;color:#111827;font:13px/1.4 ui-monospace,Menlo,Consolas,monospace;word-break:break-all;">%s</td></tr>`,
			esc(k), esc(v))
	}

	return fmt.Sprintf(`<!doctype html>
<html><body style="margin:0;padding:0;background:#f3f4f6;">
<div style="max-width:640px;margin:24px auto;background:#ffffff;border-radius:8px;overflow:hidden;border:1px solid #e5e7eb;">
  <div style="background:%s;color:#ffffff;padding:14px 20px;font:600 14px/1.2 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;letter-spacing:.04em;text-transform:uppercase;">%s %s &middot; %s</div>
  <div style="padding:20px;">
    <div style="font:600 18px/1.3 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;color:#111827;margin:0 0 14px 0;">%s</div>
    <table cellpadding="0" cellspacing="0" border="0" style="border-collapse:collapse;width:100%%;background:#f9fafb;border-radius:6px;">
      %s%s%s%s%s%s
    </table>
    <pre style="margin:18px 0 0 0;padding:14px;background:#0b1020;color:#e5e7eb;border-radius:6px;font:12px/1.5 ui-monospace,Menlo,Consolas,monospace;white-space:pre-wrap;word-break:break-word;overflow-x:auto;">%s</pre>
  </div>
  <div style="padding:12px 20px;border-top:1px solid #e5e7eb;color:#6b7280;font:12px/1.4 -apple-system,Segoe UI,Helvetica,Arial,sans-serif;">Sent by <a href="https://kpulse.io" style="color:#6b7280;text-decoration:underline;">kpulse</a> &middot; reply to your operations channel, not this address.</div>
</div>
</body></html>`,
		color, icon, esc(bannerLabel), esc(a.Cluster),
		esc(a.Title),
		row("Monitor", a.Monitor),
		row("Namespace", a.Namespace),
		row("Object", a.Object()),
		row("Reason", a.Reason),
		row("Severity", a.Severity.String()),
		row("FiredAt", a.FiredAt.Format("2006-01-02 15:04:05 MST")),
		esc(a.Body),
	)
}

func severityColor(s alert.Severity) string {
	switch s {
	case alert.Critical:
		return "#dc2626"
	case alert.Warning:
		return "#d97706"
	default:
		return "#2563eb"
	}
}

func severityIcon(s alert.Severity) string {
	switch s {
	case alert.Critical:
		return "[!!]"
	case alert.Warning:
		return "[!]"
	default:
		return "[i]"
	}
}

func truncateSubject(s string) string {
	if len(s) <= subjectMaxLen {
		return s
	}
	return s[:subjectMaxLen-3] + "..."
}

func safeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		out = "cluster"
	}
	return out
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "attachment.bin"
	}
	out := strings.ReplaceAll(name, `"`, "")
	out = strings.ReplaceAll(out, "\r", "")
	out = strings.ReplaceAll(out, "\n", "")
	return out
}
