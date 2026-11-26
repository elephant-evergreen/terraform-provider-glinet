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

func NewRoutePolicyResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "route_policy",
		Description: "Manage the stock GL.iNet policy routing global settings. Individual policy rules are not managed by this resource yet.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable policy routing."},
			"mode":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("0"), MarkdownDescription: "Policy routing mode."},
			"append_source_if": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Interface appended as the source match."},
			"instance_on":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Apply per VPN instance."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "route_policy", Type: "policy", Name: "global", Options: map[string]string{
				"enabled":          boolString(boolOf(values, "enabled", false)),
				"mode":             stringOf(values, "mode"),
				"append_source_if": stringOf(values, "append_source_if"),
				"instance_on":      boolString(boolOf(values, "instance_on", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "route_policy", Type: "policy", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":          boolValue(section.Options["enabled"] == "1"),
				"mode":             stringValue(defaultString(section.Options["mode"], "0")),
				"append_source_if": stringValue(section.Options["append_source_if"]),
				"instance_on":      boolValue(section.Options["instance_on"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "route_policy" {
				return nil, fmt.Errorf("import ID must be route_policy")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "route_policy", nil },
	})
}
