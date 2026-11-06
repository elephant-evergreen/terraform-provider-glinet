package router

import (
	"context"
	"testing"
)

func TestReadSectionResolvesSingletonAnonymousType(t *testing.T) {
	client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) {
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
	client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) {
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
