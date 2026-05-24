// Package alert defines the canonical Alert struct kpulse passes between
// detectors, the engine, and notifiers.
package alert

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Severity int

const (
	Info Severity = iota
	Warning
	Critical
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Critical:
		return "critical"
	}
	return "unknown"
}

func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info":
		return Info, nil
	case "warning", "warn":
		return Warning, nil
	case "critical", "crit":
		return Critical, nil
	}
	return Info, fmt.Errorf("unknown severity %q", s)
}

type Attachment struct {
	Name        string
	ContentType string
	Body        []byte
}

type State int

const (
	StateFiring State = iota
	StateResolved
)

func (s State) String() string {
	if s == StateResolved {
		return "resolved"
	}
	return "firing"
}

type Alert struct {
	Monitor     string
	Severity    Severity
	State       State
	Cluster     string
	Namespace   string
	ObjectKind  string
	ObjectName  string
	Reason      string
	Title       string
	Body        string
	Attachments []Attachment
	FiredAt     time.Time
}

func (a *Alert) EnsureFiredAt() {
	if a.FiredAt.IsZero() {
		a.FiredAt = time.Now().UTC()
	}
}

func (a Alert) Key() string {
	raw := strings.Join([]string{a.Monitor, a.Namespace, a.ObjectKind, a.ObjectName, a.Reason}, "|")
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (a Alert) Object() string {
	if a.ObjectKind == "" {
		return a.ObjectName
	}
	return strings.ToLower(a.ObjectKind) + "/" + a.ObjectName
}
