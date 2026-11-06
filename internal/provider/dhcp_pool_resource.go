package provider

import (
	"fmt"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewDHCPPoolResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dhcp_pool",
		Description: "Manage one OpenWrt `dhcp` pool section. The UCI section name is the stable identity; stock pools such as `lan` can be imported directly.",
		Attributes: map[string]schema.Attribute{
			"section":    schema.StringAttribute{Required: true, MarkdownDescription: "UCI dhcp section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"interface":  schema.StringAttribute{Required: true, MarkdownDescription: "Network interface served by this pool."},
			"start":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(100), MarkdownDescription: "First host offset."},
			"limit":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(150), MarkdownDescription: "Number of dynamic leases."},
			"lease_time": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("12h"), MarkdownDescription: "UCI `leasetime`."},
			"dhcpv4":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("server"), MarkdownDescription: "IPv4 DHCP mode."},
			"dhcpv6":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("disabled"), MarkdownDescription: "IPv6 DHCP mode."},
			"ra":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("disabled"), MarkdownDescription: "Router advertisement mode."},
			"force":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Force the DHCP server to start."},
			"ignore":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Disable this pool."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section := stringOf(values, "section")
			if section == "" {
				return router.Section{}, fmt.Errorf("section is required")
			}
			options := map[string]string{
				"interface": stringOf(values, "interface"),
				"start":     fmt.Sprintf("%d", int64Of(values, "start")),
				"limit":     fmt.Sprintf("%d", int64Of(values, "limit")),
				"leasetime": stringOf(values, "lease_time"),
				"dhcpv4":    stringOf(values, "dhcpv4"),
				"dhcpv6":    stringOf(values, "dhcpv6"),
				"ra":        stringOf(values, "ra"),
				"force":     boolString(boolOf(values, "force", true)),
			}
			if boolOf(values, "ignore", false) {
				options["ignore"] = "1"
			}
			return router.Section{Package: "dhcp", Type: "dhcp", Name: section, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			start, _ := strconv.ParseInt(section.Options["start"], 10, 64)
			limit, _ := strconv.ParseInt(section.Options["limit"], 10, 64)
			return map[string]attr.Value{
				"section":    stringValue(section.Name),
				"interface":  stringValue(section.Options["interface"]),
				"start":      int64Value(start),
				"limit":      int64Value(limit),
				"lease_time": stringValue(defaultString(section.Options["leasetime"], "12h")),
				"dhcpv4":     stringValue(defaultString(section.Options["dhcpv4"], "server")),
				"dhcpv6":     stringValue(defaultString(section.Options["dhcpv6"], "disabled")),
				"ra":         stringValue(defaultString(section.Options["ra"], "disabled")),
				"force":      boolValue(section.Options["force"] != "0"),
				"ignore":     boolValue(section.Options["ignore"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a dhcp section name")
			}
			return map[string]attr.Value{"section": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "section"), nil },
		Validate: func(values map[string]attr.Value) error {
			start, limit := int64Of(values, "start"), int64Of(values, "limit")
			if stringOf(values, "interface") == "" {
				return fmt.Errorf("interface is required")
			}
			if start < 1 || limit < 1 || start+limit > 255 {
				return fmt.Errorf("start and limit must keep the pool inside the /24")
			}
			return nil
		},
	})
}
