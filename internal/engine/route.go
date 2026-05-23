package engine

import (
	"github.com/dnl555/kpulse/internal/alert"
	"github.com/dnl555/kpulse/internal/config"
)

type Router struct {
	rules    []config.RoutingRule
	defaults []string
}

func NewRouter(rules []config.RoutingRule, defaults []string) *Router {
	return &Router{rules: rules, defaults: defaults}
}

func (r *Router) Channels(a alert.Alert) []string {
	for _, rule := range r.rules {
		if matchRule(rule.Match, a) {
			return rule.Channels
		}
	}
	return r.defaults
}

func matchRule(m config.RoutingMatch, a alert.Alert) bool {
	if m.Severity == "" && m.Monitor == "" {
		return false
	}
	if m.Severity != "" && m.Severity != a.Severity.String() {
		return false
	}
	if m.Monitor != "" && m.Monitor != a.Monitor {
		return false
	}
	return true
}
