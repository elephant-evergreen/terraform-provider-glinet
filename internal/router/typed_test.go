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
