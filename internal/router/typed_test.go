package router

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestForgetSectionClearsOwnerToken(t *testing.T) {
	options := map[string]string{"hostname": "router", ManagedOption: "1", OwnerTokenOption: "token"}
	state := uciState{"system": {{Name: "system", Type: "system", Options: options}}}
	current := Configuration{{Package: "system", Name: "system", Type: "system", Options: options}}
	var write string
	client := newFixtureClient(runnerFunc(func(_ context.Context, script string) ([]byte, error) {
		switch {
		case strings.Contains(script, safetyStateMarker):
			return json.Marshal(state)
		case strings.Contains(script, "keys_match"):
			return json.Marshal(map[string]any{"configuration": current, "keys_match": true})
		case strings.Contains(script, "io.write(json.encode(result))"):
			return json.Marshal(current)
		case strings.Contains(script, "GLINET_REQUEST"):
			write = script
			return nil, &ExitError{Code: 75}
		}
		return []byte("missing"), nil
	}))
	if err := client.ForgetSection(context.Background(), Section{Package: "system", Name: "system", Type: "system", Options: map[string]string{"hostname": "router"}}, nil); err == nil {
		t.Fatal("forget did not reach the transaction")
	}
	if write == "" {
		t.Fatal("forget did not write the section")
	}
	if !strings.Contains(write, `"terraform_owner":"0"`) {
		t.Fatalf("forget did not release the marker:\n%s", write)
	}
	if !strings.Contains(write, `"terraform_owner_id":""`) {
		t.Fatalf("forget did not clear the owner token on the router:\n%s", write)
	}
}

func TestAnonymousSectionNameMatchesCreateName(t *testing.T) {
	selector := Section{Package: "firewall", Type: "redirect", Name: "@redirect[0]", Match: map[string]string{"name": "Acceptance"}}
	name := AnonymousSectionName(selector.Package, selector.Type, selector.Match)
	if !strings.HasPrefix(name, "tf_glinet_") || len(name) != len("tf_glinet_")+16 {
		t.Fatalf("unexpected anonymous section name %q", name)
	}
	if !strings.Contains(configurationLua(Configuration{selector}, true), name) {
		t.Fatalf("the write script does not create %s", name)
	}
	if other := AnonymousSectionName(selector.Package, selector.Type, map[string]string{"name": "Other"}); other == name {
		t.Fatal("the identity is not part of the create name")
	}
	if singleton := AnonymousSectionName("dropbear", "dropbear", nil); singleton == name {
		t.Fatal("different selectors produced the same create name")
	}
}
