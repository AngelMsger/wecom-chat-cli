package config

import "testing"

func TestPolicyIsDenyByDefaultAndBounded(t *testing.T) {
	p := Example()
	if p.Validate() != nil || p.ReplyMode != "review" || len(p.AllowedUsers) != 0 || len(p.AllowedGroups) != 0 {
		t.Fatal("unsafe defaults")
	}
	for _, mutate := range []func(*Policy){func(p *Policy) { p.AllowedUsers = []string{"*"} }, func(p *Policy) { p.ReplyTTLSeconds = 301 }, func(p *Policy) { p.MaxEvents = 0 }, func(p *Policy) { p.OperatorSocket = p.AgentSocket }, func(p *Policy) { p.StateDir = "relative" }, func(p *Policy) { p.AllowedUsers = []string{"alice", "alice"} }} {
		p := Example()
		mutate(&p)
		if p.Validate() == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	for _, change := range []func(*Policy){func(p *Policy) { p.Version = 1 }, func(p *Policy) { p.ReplyMode = "auto" }, func(p *Policy) { p.ReplyMode = "template" }, func(p *Policy) { p.TaskTTLSeconds = p.ReplyTTLSeconds + 1 }, func(p *Policy) { p.Templates = map[string]string{"bad": "\x1b[2J"} }} {
		p := Example()
		change(&p)
		if p.Validate() == nil {
			t.Fatal("unsafe task/send policy accepted")
		}
	}
}
