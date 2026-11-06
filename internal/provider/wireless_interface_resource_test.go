package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWirelessStatePreservesKeyVersion(t *testing.T) {
	r := newTypedResource(typedWirelessInterfaceSpec()).(*typedResource)
	var schema frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schema)
	actual := router.Section{Package: "wireless", Type: "wifi-iface", Name: "wifi2g", Options: map[string]string{"ssid": "Example", "mode": "ap", "encryption": "psk2", "disabled": "0", "isolate": "0"}}
	planned := map[string]attr.Value{
		"section":     types.StringValue("wifi2g"),
		"key_wo":      types.StringNull(),
		"key_version": types.Int64Value(3),
	}
	object, diagnostics := r.stateObject(planned, actual)
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if object.Attributes()["key_version"].(types.Int64).ValueInt64() != 3 {
		t.Fatalf("key_version was overwritten: %+v", object.Attributes()["key_version"])
	}
}
