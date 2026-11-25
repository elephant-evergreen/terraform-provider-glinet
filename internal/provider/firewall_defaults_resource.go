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

func NewFirewallDefaultsResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "firewall_defaults",
		Description: "Manage the singleton OpenWrt `firewall` defaults section. Stock firewall includes remain outside this resource. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"input":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"), MarkdownDescription: "Default input policy: ACCEPT, REJECT, or DROP."},
			"output":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ACCEPT"), MarkdownDescription: "Default output policy: ACCEPT, REJECT, or DROP."},
			"forward":            schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"), MarkdownDescription: "Default forward policy: ACCEPT, REJECT, or DROP."},
			"syn_flood":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable SYN flood protection."},
			"synflood_protect":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable the SYN flood protection rule."},
			"synflood_rate":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("25/s"), MarkdownDescription: "SYN flood rate limit."},
			"synflood_burst":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("50"), MarkdownDescription: "SYN flood burst limit."},
			"drop_invalid":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Drop invalid packets."},
			"tcp_syncookies":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable TCP SYN cookies."},
			"flow_offloading":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable software flow offloading."},
			"flow_offloading_hw": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable hardware flow offloading."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{
				Package: "firewall", Type: "defaults", Name: "@defaults[0]",
				Options: map[string]string{
					"input":              stringOf(values, "input"),
					"output":             stringOf(values, "output"),
					"forward":            stringOf(values, "forward"),
					"syn_flood":          boolString(boolOf(values, "syn_flood", true)),
					"synflood_protect":   boolString(boolOf(values, "synflood_protect", true)),
					"synflood_rate":      stringOf(values, "synflood_rate"),
					"synflood_burst":     stringOf(values, "synflood_burst"),
					"drop_invalid":       boolString(boolOf(values, "drop_invalid", false)),
					"tcp_syncookies":     boolString(boolOf(values, "tcp_syncookies", true)),
					"flow_offloading":    boolString(boolOf(values, "flow_offloading", false)),
					"flow_offloading_hw": boolString(boolOf(values, "flow_offloading_hw", false)),
				},
			}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "firewall", Type: "defaults", Name: "@defaults[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"input":              stringValue(defaultString(section.Options["input"], "REJECT")),
				"output":             stringValue(defaultString(section.Options["output"], "ACCEPT")),
				"forward":            stringValue(defaultString(section.Options["forward"], "REJECT")),
				"syn_flood":          boolValue(section.Options["syn_flood"] != "0"),
				"synflood_protect":   boolValue(section.Options["synflood_protect"] != "0"),
				"synflood_rate":      stringValue(defaultString(section.Options["synflood_rate"], "25/s")),
				"synflood_burst":     stringValue(defaultString(section.Options["synflood_burst"], "50")),
				"drop_invalid":       boolValue(section.Options["drop_invalid"] == "1"),
				"tcp_syncookies":     boolValue(section.Options["tcp_syncookies"] != "0"),
				"flow_offloading":    boolValue(section.Options["flow_offloading"] == "1"),
				"flow_offloading_hw": boolValue(section.Options["flow_offloading_hw"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "firewall_defaults" {
				return nil, fmt.Errorf("import ID must be firewall_defaults")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "firewall_defaults", nil },
		Validate: func(values map[string]attr.Value) error {
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
