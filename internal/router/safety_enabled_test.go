package router

import "testing"

func TestFirewallEnablePredicateUsesEnabled(t *testing.T) {
	config := Configuration{{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{"device": "br-lan"}}}
	state := func(options map[string]string) uciState {
		return uciState{"firewall": []uciSection{{Name: "cfg1", Type: "rule", Options: options}}}
	}
	if err := checkTopologyOwnership(config, state(map[string]string{"target": "ACCEPT", "dest": "wan"})); err == nil {
		t.Fatal("an enabled ACCEPT rule to wan was accepted")
	}
	if err := checkTopologyOwnership(config, state(map[string]string{"target": "ACCEPT", "dest": "wan", "enabled": "0"})); err != nil {
		t.Fatalf("a disabled ACCEPT rule was rejected: %v", err)
	}
	if err := checkTopologyOwnership(config, state(map[string]string{"target": "ACCEPT", "dest": "wan", "disabled": "1"})); err == nil {
		t.Fatal("a rule carrying only the ignored disabled option was accepted")
	}
}
