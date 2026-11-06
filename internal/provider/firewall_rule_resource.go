package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewFirewallRuleResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "firewall_rule",
		Description: "Manage one firewall rule by its unique `name` option. Stock rules can be adopted by name; new rules are created as named sections. `position` requests an absolute UCI rule index.",
		Attributes: map[string]schema.Attribute{
			"name":       schema.StringAttribute{Required: true, MarkdownDescription: "Unique rule name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"src":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Source zone, `*`, or empty."},
			"dest":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Destination zone, `*`, or empty."},
			"src_ip":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Source address or CIDR."},
			"dest_ip":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Destination address or CIDR."},
			"protocols":  schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Protocols, such as `tcp`, `udp`, `icmp`, or `all`."},
			"src_ports":  schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Source ports or ranges."},
			"dest_ports": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Destination ports or ranges."},
			"family":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ipv4"), MarkdownDescription: "Address family: `ipv4`, `ipv6`, or `any`."},
			"target":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"), MarkdownDescription: "Target: ACCEPT, REJECT, DROP, or LOG."},
			"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the rule is active."},
			"position":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Absolute UCI firewall rule index. Zero leaves ordering unspecified."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"name": name, "family": stringOf(values, "family"), "target": stringOf(values, "target"), "enabled": boolString(boolOf(values, "enabled", true)),
			}
			for _, field := range []struct{ attr, option string }{{"src", "src"}, {"dest", "dest"}, {"src_ip", "src_ip"}, {"dest_ip", "dest_ip"}} {
				if value := stringOf(values, field.attr); value != "" {
					options[field.option] = value
				}
			}
			if protocols := stringsOf(values, "protocols"); len(protocols) > 0 {
				options["proto"] = joinStrings(protocols)
			}
			if ports := stringsOf(values, "src_ports"); len(ports) > 0 {
				options["src_port"] = joinStrings(ports)
			}
			if ports := stringsOf(values, "dest_ports"); len(ports) > 0 {
				options["dest_port"] = joinStrings(ports)
			}
			section := router.Section{Package: "firewall", Type: "rule", Name: "@rule[0]", Match: map[string]string{"name": name}, Options: options}
			if position := int64Of(values, "position"); position > 0 {
				value := int(position)
				section.Position = &value
			}
			return section, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			position := int64(0)
			if section.Position != nil {
				position = int64(*section.Position)
			}
			name := section.Options["name"]
			if name == "" {
				name = section.Name
			}
			return map[string]attr.Value{
				"name":       stringValue(name),
				"src":        stringValue(section.Options["src"]),
				"dest":       stringValue(section.Options["dest"]),
				"src_ip":     stringValue(section.Options["src_ip"]),
				"dest_ip":    stringValue(section.Options["dest_ip"]),
				"protocols":  stringSet(splitFields(section.Options["proto"])),
				"src_ports":  stringSet(splitFields(section.Options["src_port"])),
				"dest_ports": stringSet(splitFields(section.Options["dest_port"])),
				"family":     stringValue(defaultString(section.Options["family"], "ipv4")),
				"target":     stringValue(defaultString(section.Options["target"], "REJECT")),
				"enabled":    boolValue(section.Options["enabled"] != "0"),
				"position":   int64Value(position),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a rule name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" {
				return fmt.Errorf("name is required")
			}
			switch stringOf(values, "target") {
			case "ACCEPT", "REJECT", "DROP", "LOG":
			default:
				return fmt.Errorf("target must be ACCEPT, REJECT, DROP, or LOG")
			}
			switch stringOf(values, "family") {
			case "ipv4", "ipv6", "any":
			default:
				return fmt.Errorf("family must be ipv4, ipv6, or any")
			}
			if position := int64Of(values, "position"); position < 0 || position > 4095 {
				return fmt.Errorf("position must be between 0 and 4095")
			}
			return nil
		},
	})
}
