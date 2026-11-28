package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPreserveOwnerKeepsKnownStateStable(t *testing.T) {
	values := map[string]attr.Value{"owner": types.StringUnknown()}
	preserveOwner(values, map[string]attr.Value{"owner": types.StringValue("token")})
	if got := values["owner"].(types.String).ValueString(); got != "token" {
		t.Fatalf("owner = %q, want token", got)
	}
}

func TestPreserveOwnerLeavesLegacyStateUnknown(t *testing.T) {
	for name, state := range map[string]types.String{"null": types.StringNull(), "empty": types.StringValue("")} {
		t.Run(name, func(t *testing.T) {
			values := map[string]attr.Value{"owner": types.StringUnknown()}
			preserveOwner(values, map[string]attr.Value{"owner": state})
			if !values["owner"].IsUnknown() {
				t.Fatal("a legacy state without an owner must plan the owner as unknown so the applied token is consistent")
			}
		})
	}
}
