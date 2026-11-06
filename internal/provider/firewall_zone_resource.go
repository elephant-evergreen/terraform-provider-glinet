package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewFirewallZoneResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "firewall_zone",
		Description: "Manage one OpenWrt firewall zone. Zones are resolved by their `name` option, so stock anonymous zones can be adopted safely. Destroy releases ownership by default.",
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true, MarkdownDescription: "Zone name, such as `lan` or `wan`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"networks": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Interfaces classified into this zone."},
			"input":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"), MarkdownDescription: "Input policy: ACCEPT, REJECT, DROP."},
			"output":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ACCEPT"), MarkdownDescription: "Output policy."},
			"forward":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"), MarkdownDescription: "Forward policy."},
			"masq":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable masquerading."},
			"mtu_fix":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable MSS clamping."},
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the zone is enabled."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			section := router.Section{Package: "firewall", Type: "zone", Name: "@zone[0]", Match: map[string]string{"name": name}, Options: map[string]string{
				"name": name, "input": stringOf(values, "input"), "output": stringOf(values, "output"), "forward": stringOf(values, "forward"),
				"masq": boolString(boolOf(values, "masq", false)), "mtu_fix": boolString(boolOf(values, "mtu_fix", false)), "enabled": boolString(boolOf(values, "enabled", true)),
			}}
			if networks := stringsOf(values, "networks"); len(networks) > 0 {
				section.Lists = map[string][]string{"network": networks}
			}
			return section, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":     stringValue(section.Options["name"]),
				"networks": stringSet(section.Lists["network"]),
				"input":    stringValue(defaultString(section.Options["input"], "REJECT")),
				"output":   stringValue(defaultString(section.Options["output"], "ACCEPT")),
				"forward":  stringValue(defaultString(section.Options["forward"], "REJECT")),
				"masq":     boolValue(section.Options["masq"] == "1"),
				"mtu_fix":  boolValue(section.Options["mtu_fix"] == "1"),
				"enabled":  boolValue(section.Options["enabled"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a zone name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" {
				return fmt.Errorf("name is required")
			}
			for _, field := range []string{"input", "output", "forward"} {
				switch stringOf(values, field) {
				case "ACCEPT", "REJECT", "DROP":
				default:
					return fmt.Errorf("%s must be ACCEPT, REJECT, or DROP", field)
				}
			}
			return nil
		},
	})
}
