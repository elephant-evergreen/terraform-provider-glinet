package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
)

func NewZeroTierResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "zerotier",
		Description: "Manage the stock ZeroTier integration. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable ZeroTier."},
			"nat":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable NAT for the ZeroTier network."},
			"wan_nat": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable WAN NAT."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "zerotier", Type: "zerotier", Name: "gl", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "nat": boolString(boolOf(values, "nat", false)), "wan_nat": boolString(boolOf(values, "wan_nat", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "zerotier", Type: "zerotier", Name: "gl"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"enabled": boolValue(section.Options["enabled"] == "1"), "nat": boolValue(section.Options["nat"] == "1"), "wan_nat": boolValue(section.Options["wan_nat"] == "1")}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "zerotier" {
				return nil, fmt.Errorf("import ID must be zerotier")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "zerotier", nil },
	})
}
