package notifiers

import (
	"context"
	"errors"
	"testing"

	"github.com/dnl555/kpulse/internal/alert"
)

type stub struct {
	name string
	sent []alert.Alert
	err  error
}

func (s *stub) Name() string { return s.name }
func (s *stub) Send(_ context.Context, a alert.Alert) error {
	s.sent = append(s.sent, a)
	return s.err
}

func TestRegistryDispatch(t *testing.T) {
	r := NewRegistry()
	a := &stub{name: "slack"}
	b := &stub{name: "email"}
	r.Register(a)
	r.Register(b)

	if err := r.Send(context.Background(), alert.Alert{Monitor: "x"}, []string{"slack"}); err != nil {
		t.Fatal(err)
	}
	if len(a.sent) != 1 || len(b.sent) != 0 {
		t.Errorf("expected only slack to get alert; slack=%d email=%d", len(a.sent), len(b.sent))
	}
}

func TestRegistryAggregateErrors(t *testing.T) {
	r := NewRegistry()
	r.Register(&stub{name: "slack", err: errors.New("boom")})
	r.Register(&stub{name: "email"})
	err := r.Send(context.Background(), alert.Alert{}, []string{"slack", "email"})
	if err == nil {
		t.Fatal("expected aggregated error")
	}
}

func TestRegistryUnknownChannel(t *testing.T) {
	r := NewRegistry()
	r.Register(&stub{name: "slack"})
	if err := r.Send(context.Background(), alert.Alert{}, []string{"nope"}); err == nil {
		t.Error("expected error for unknown channel")
	}
}
