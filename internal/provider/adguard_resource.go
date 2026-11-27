package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
)

func NewAdGuardResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "adguard",
		Description: "Manage the stock AdGuard Home integration switch. The AdGuard Home YAML configuration is not managed by this resource.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable AdGuard Home integration."},
			"override_vpn": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Let AdGuard Home DNS bypass VPN DNS."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "adguardhome", Type: "adguardhome", Name: "config", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "override_vpn": boolString(boolOf(values, "override_vpn", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "adguardhome", Type: "adguardhome", Name: "config"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enabled"] == "1"), "override_vpn": boolValue(section.Options["override_vpn"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "adguard" {
				return nil, fmt.Errorf("import ID must be adguard")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "adguard", nil },
	})
}
