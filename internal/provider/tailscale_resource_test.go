package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"testing"
)

func TestTailscaleSpecRoundTrip(t *testing.T) {
	spec := NewTailscaleResource().(*typedResource).spec
	values := map[string]attr.Value{"port": types.Int64Value(41641), "log_stdout": types.BoolValue(true), "log_stderr": types.BoolValue(true), "state_file": types.StringValue("/etc/tailscale/tailscaled.state")}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "tailscale" || section.Options["port"] != "41641" {
		t.Fatalf("unexpected Tailscale: %+v", section)
	}
}
