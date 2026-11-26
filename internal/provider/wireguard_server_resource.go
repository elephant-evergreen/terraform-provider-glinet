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

func NewWireGuardServerResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "wireguard_server",
		Description: "Manage one stock GL.iNet WireGuard server. Client configuration and key distribution remain in the stock UI.",
		Attributes: map[string]schema.Attribute{
			"name":             schema.StringAttribute{Required: true, MarkdownDescription: "Server section name, normally `main_server`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"address_v4":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "IPv4 server address and prefix."},
			"address_v6":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "IPv6 server address and prefix."},
			"port":             schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(51820), MarkdownDescription: "UDP listen port."},
			"access":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("DROP"), MarkdownDescription: "Default access policy: ACCEPT, REJECT, or DROP."},
			"client_to_client": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Allow clients to reach each other."},
			"masq":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable masquerading for server traffic."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			return router.Section{Package: "wireguard_server", Type: "servers", Name: name, Options: map[string]string{
				"address_v4":       stringOf(values, "address_v4"),
				"address_v6":       stringOf(values, "address_v6"),
				"port":             strconv.FormatInt(int64Of(values, "port"), 10),
				"access":           stringOf(values, "access"),
				"client_to_client": boolString(boolOf(values, "client_to_client", false)),
				"masq":             boolString(boolOf(values, "masq", true)),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":             stringValue(section.Name),
				"address_v4":       stringValue(section.Options["address_v4"]),
				"address_v6":       stringValue(section.Options["address_v6"]),
				"port":             int64Value(parseOptionInt64(section.Options["port"])),
				"access":           stringValue(defaultString(section.Options["access"], "DROP")),
				"client_to_client": boolValue(section.Options["client_to_client"] == "1"),
				"masq":             boolValue(section.Options["masq"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a wireguard_server section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			switch stringOf(values, "access") {
			case "ACCEPT", "REJECT", "DROP":
			default:
				return fmt.Errorf("access must be ACCEPT, REJECT, or DROP")
			}
			if port := int64Of(values, "port"); port < 1 || port > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
			return nil
		},
	})
}

func NewS2SResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "s2s",
		Description: "Manage the stock GL.iNet site-to-site switch.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable site-to-site networking."},
			"firewall": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Apply site-to-site firewall rules."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_s2s", Type: "global", Name: "global", Options: map[string]string{
				"enable":   boolString(boolOf(values, "enabled", false)),
				"firewall": boolString(boolOf(values, "firewall", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_s2s", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":  boolValue(section.Options["enable"] == "1"),
				"firewall": boolValue(section.Options["firewall"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "s2s" {
				return nil, fmt.Errorf("import ID must be s2s")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "s2s", nil },
	})
}
