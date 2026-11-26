package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewTorResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "tor",
		Description: "Manage the stock Tor integration. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable Tor."},
			"manual":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Use manually selected exit nodes."},
			"countries":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("all"), MarkdownDescription: "Exit node countries."},
			"ipv6_intercept": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Intercept IPv6 traffic."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "tor", Type: "global", Name: "global", Options: map[string]string{
				"enable": boolString(boolOf(values, "enabled", false)), "manual": boolString(boolOf(values, "manual", false)),
				"countries": stringOf(values, "countries"), "ipv6_intercept": boolString(boolOf(values, "ipv6_intercept", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "tor", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"enabled": boolValue(section.Options["enable"] == "1"), "manual": boolValue(section.Options["manual"] == "1"), "countries": stringValue(defaultString(section.Options["countries"], "all")), "ipv6_intercept": boolValue(section.Options["ipv6_intercept"] != "0")}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "tor" {
				return nil, fmt.Errorf("import ID must be tor")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "tor", nil },
	})
}
