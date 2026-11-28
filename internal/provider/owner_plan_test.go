package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPreserveOwnerKeepsLegacyStateStable(t *testing.T) {
	values := map[string]attr.Value{"owner": types.StringUnknown()}
	preserveOwner(values, map[string]attr.Value{"owner": types.StringNull()})
	if !values["owner"].IsNull() {
		t.Fatal("a legacy state with no owner must keep the plan null")
	}

	values = map[string]attr.Value{"owner": types.StringUnknown()}
	preserveOwner(values, map[string]attr.Value{"owner": types.StringValue("token")})
	if got := values["owner"].(types.String).ValueString(); got != "token" {
		t.Fatalf("owner = %q, want token", got)
	}
}
