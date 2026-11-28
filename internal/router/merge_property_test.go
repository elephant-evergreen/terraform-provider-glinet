package router

import (
	"fmt"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// TestMergeManagedConfigurationPreservesUnchangedSections models a write as a
// set of changes on top of the managed configuration: an unchanged section must
// stay byte-identical, an update must replace it exactly once, an absent change
// must remove it, and a new section must appear once.
func TestMergeManagedConfigurationPreservesUnchangedSections(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 6).Draw(t, "sections")
		full := Configuration{}
		for index := 0; index < count; index++ {
			name := fmt.Sprintf("rule_%d", index)
			full = append(full, Section{Package: "firewall", Name: name, Type: "rule", Options: map[string]string{"name": "name-" + name, ManagedOption: "1"}})
		}
		changes := Configuration{}
		updated := map[string]bool{}
		absent := map[string]bool{}
		for _, section := range full {
			switch rapid.IntRange(0, 2).Draw(t, "operation-"+section.Name) {
			case 1:
				updated[section.Name] = true
				changes = append(changes, Section{Package: "firewall", Name: section.Name, Type: "rule", Options: map[string]string{"name": "updated-" + section.Name, ManagedOption: "1"}})
			case 2:
				absent[section.Name] = true
				changes = append(changes, Section{Package: "firewall", Name: section.Name, Type: "rule", Absent: true})
			}
		}
		addNew := rapid.Bool().Draw(t, "new-section")
		if addNew {
			changes = append(changes, Section{Package: "firewall", Name: "rule_new", Type: "rule", Options: map[string]string{"name": "new", ManagedOption: "1"}})
		}
		merged := mergeManagedConfiguration(full, changes)
		byName := map[string]Section{}
		for _, section := range merged {
			if _, ok := byName[section.Name]; ok {
				t.Fatalf("merged configuration contains %s twice", section.Name)
			}
			byName[section.Name] = section
		}
		for _, section := range full {
			got, present := byName[section.Name]
			switch {
			case absent[section.Name]:
				if present {
					t.Fatalf("absent change kept %s", section.Name)
				}
			case updated[section.Name]:
				if !present || got.Options["name"] != "updated-"+section.Name {
					t.Fatalf("update did not replace %s exactly: %#v", section.Name, got)
				}
			default:
				if !present || !reflect.DeepEqual(got, section) {
					t.Fatalf("unchanged section %s changed: %#v -> %#v", section.Name, section, got)
				}
			}
		}
		if addNew {
			if got, present := byName["rule_new"]; !present || got.Options["name"] != "new" {
				t.Fatalf("new section was not added: %#v", got)
			}
		}
	})
}
