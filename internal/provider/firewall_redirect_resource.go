package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewFirewallRedirectResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "firewall_redirect",
		Description: "Manage one firewall redirect (destination NAT). Unmanaged redirects are rejected by the provider's safety checks; a redirect declared by this resource is treated as managed. Destroy releases ownership by default.",
		Attributes: map[string]schema.Attribute{
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "Unique redirect name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"src":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Source zone."},
			"src_dport":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Source destination port."},
			"dest":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Destination zone."},
			"dest_ip":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Internal destination address."},
			"dest_port":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Internal destination port."},
			"proto":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("tcp"), MarkdownDescription: "Protocol."},
			"target":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("DNAT"), MarkdownDescription: "Target, normally DNAT."},
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the redirect is active."},
			"reflection":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable NAT reflection."},
			"reflection_src": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Reflection source: `internal`, `external`, or empty."},
			"helper":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional firewall helper."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"name": name, "src": stringOf(values, "src"), "src_dport": stringOf(values, "src_dport"),
				"src_ip": stringOf(values, "src_ip"), "src_mac": stringOf(values, "src_mac"),
				"dest": stringOf(values, "dest"), "dest_ip": stringOf(values, "dest_ip"), "dest_port": stringOf(values, "dest_port"),
				"proto": stringOf(values, "proto"), "target": stringOf(values, "target"), "enabled": boolString(boolOf(values, "enabled", true)),
			}
			options["reflection"] = boolString(boolOf(values, "reflection", false))
			options["reflection_src"] = stringOf(values, "reflection_src")
			options["helper"] = stringOf(values, "helper")
			return router.Section{Package: "firewall", Type: "redirect", Name: "@redirect[0]", Match: map[string]string{"name": name}, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":           stringValue(section.Options["name"]),
				"src":            stringValue(section.Options["src"]),
				"src_dport":      stringValue(section.Options["src_dport"]),
				"dest":           stringValue(section.Options["dest"]),
				"dest_ip":        stringValue(section.Options["dest_ip"]),
				"dest_port":      stringValue(section.Options["dest_port"]),
				"proto":          stringValue(defaultString(section.Options["proto"], "tcp")),
				"target":         stringValue(defaultString(section.Options["target"], "DNAT")),
				"enabled":        boolValue(section.Options["enabled"] != "0"),
				"reflection":     boolValue(section.Options["reflection"] == "1"),
				"reflection_src": stringValue(section.Options["reflection_src"]),
				"helper":         stringValue(section.Options["helper"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a redirect name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" {
				return fmt.Errorf("name is required")
			}
			if stringOf(values, "target") != "DNAT" {
				return fmt.Errorf("target must be DNAT")
			}
			return nil
		},
	})
}
