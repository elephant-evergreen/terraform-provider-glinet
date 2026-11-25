package router

import (
	"context"
	"testing"
)

func TestReadSectionResolvesSingletonAnonymousType(t *testing.T) {
	client := newFixtureClient(runnerFunc(func(context.Context, string) ([]byte, error) {
		return []byte(`{"gl-dns-v2":[{"name":"cfg123","type":"dns","options":{"provider":"quad9"}}]}`), nil
	}))
	actual, found, err := client.ReadSection(context.Background(), Section{Package: "gl-dns-v2", Type: "dns", Name: "@dns[0]"})
	if err != nil {
		t.Fatal(err)
	}
	if !found || actual.Name != "cfg123" {
		t.Fatalf("singleton section was not resolved: found=%v actual=%+v", found, actual)
	}
}

func TestReadSectionUsesFullFirewallPackageForPosition(t *testing.T) {
	client := newFixtureClient(runnerFunc(func(context.Context, string) ([]byte, error) {
		return []byte(`{"firewall":[{"name":"@defaults[0]","type":"defaults"},{"name":"@zone[0]","type":"zone"},{"name":"@forwarding[0]","type":"forwarding"},{"name":"@rule[0]","type":"rule","options":{"name":"First"}},{"name":"@rule[1]","type":"rule","options":{"name":"Second"}}]}`), nil
	}))
	actual, found, err := client.ReadSection(context.Background(), Section{Package: "firewall", Type: "rule", Name: "@rule[0]", Match: map[string]string{"name": "First"}})
	if err != nil {
		t.Fatal(err)
	}
	if !found || actual.Name != "@rule[0]" || actual.Position == nil || *actual.Position != 3 {
		t.Fatalf("rule position did not include non-rule firewall sections: found=%v actual=%+v", found, actual)
	}
	actual, found, err = client.ReadSection(context.Background(), Section{Package: "firewall", Type: "rule", Name: "@rule[0]", Match: map[string]string{"name": "Second"}})
	if err != nil {
		t.Fatal(err)
	}
	if !found || actual.Position == nil || *actual.Position != 4 {
		t.Fatalf("second rule position = %+v, want 4", actual)
	}
}

func TestReadSectionRejectsAmbiguousAnonymousIdentity(t *testing.T) {
	for _, match := range []map[string]string{nil, {"name": "duplicate"}} {
		client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) {
			return []byte(`{"firewall":[{"name":"one","type":"zone","options":{"name":"duplicate"}},{"name":"two","type":"zone","options":{"name":"duplicate"}}]}`), nil
		}), nil)
		if _, _, err := client.ReadSection(context.Background(), Section{Package: "firewall", Name: "@zone[0]", Type: "zone", Match: match}); err == nil {
			t.Fatal("ambiguous identity was accepted")
		}
	}
}

func TestFirewallRulesReadOrder(t *testing.T) {
	client := newFixtureClient(runnerFunc(func(context.Context, string) ([]byte, error) {
		return []byte(`{"firewall":[{"name":"@defaults[0]","type":"defaults"},{"name":"@zone[0]","type":"zone"},{"name":"@rule[0]","type":"rule","options":{"name":"First"}},{"name":"@rule[1]","type":"rule","options":{"name":"Second"}}]}`), nil
	}))
	rules, err := client.FirewallRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].Name != "@rule[0]" || rules[1].Name != "@rule[1]" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
	if rules[0].Position == nil || *rules[0].Position != 2 || rules[1].Position == nil || *rules[1].Position != 3 {
		t.Fatalf("unexpected positions: %+v", rules)
	}
}

func TestApplyFirewallRulesRejectsIncompletePolicy(t *testing.T) {
	client := newFixtureClient(runnerFunc(func(context.Context, string) ([]byte, error) {
		return []byte(`{"firewall":[{"name":"@rule[0]","type":"rule","options":{"name":"First"}}]}`), nil
	}))
	err := client.ApplyFirewallRules(context.Background(), Configuration{}, Configuration{})
	if err == nil {
		t.Fatal("incomplete policy was accepted")
	}
}

func TestManagedMarkerLegacyReadAndMigration(t *testing.T) {
	state := uciState{"network": {{Name: "lan", Type: "interface", Options: map[string]string{LegacyManagedOption: "1", "device": "br-lan"}}}}
	managed := configurationFromManagedState(state)
	if len(managed) != 1 || managed[0].Name != "lan" {
		t.Fatalf("legacy marker was not read as managed: %+v", managed)
	}
	migrated := migrateLegacyMarkers(nil, managed)
	if len(migrated) != 1 || migrated[0].Options[ManagedOption] != "1" || migrated[0].Options[LegacyManagedOption] != "" {
		t.Fatalf("legacy marker was not migrated: %+v", migrated)
	}
	forget := Configuration{{Package: "network", Name: "lan", Type: "interface", Options: map[string]string{ManagedOption: "0"}}}
	migrated = migrateLegacyMarkers(forget, managed)
	if migrated[0].Options[ManagedOption] != "0" {
		t.Fatalf("explicit release was overwritten by migration: %+v", migrated)
	}
}
