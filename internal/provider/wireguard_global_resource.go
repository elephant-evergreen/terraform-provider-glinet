package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
)

func NewWireGuardGlobalResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "wireguard_global",
		Description: "Manage the stock WireGuard global proxy switch. Provider and peer profiles are separate.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"global_proxy": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Route all clients through the WireGuard proxy."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "wireguard", Type: "proxy", Name: "global", Options: map[string]string{"global_proxy": boolString(boolOf(values, "global_proxy", false))}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "wireguard", Type: "proxy", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"global_proxy": boolValue(section.Options["global_proxy"] == "1")}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "wireguard_global" {
				return nil, fmt.Errorf("import ID must be wireguard_global")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "wireguard_global", nil },
	})
}
