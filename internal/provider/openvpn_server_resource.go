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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewOpenVPNServerResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "openvpn_server",
		Description: "Manage one OpenWrt OpenVPN server section. Certificate and key file paths are managed as values; the files themselves are not managed.",
		Attributes: map[string]schema.Attribute{
			"name":    schema.StringAttribute{Required: true, MarkdownDescription: "UCI OpenVPN section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable this server."},
			"port":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1194), MarkdownDescription: "Listening port."},
			"proto":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("udp"), MarkdownDescription: "Transport protocol."},
			"dev":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("tun"), MarkdownDescription: "Device type."},
			"server":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Server subnet and netmask."},
			"ca":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "CA certificate path."},
			"cert":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Server certificate path."},
			"key":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Server key path."},
			"dh":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Diffie-Hellman parameters path."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			return router.Section{Package: "openvpn", Type: "openvpn", Name: name, Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "port": fmt.Sprintf("%d", int64Of(values, "port")),
				"proto": stringOf(values, "proto"), "dev": stringOf(values, "dev"), "server": stringOf(values, "server"),
				"ca": stringOf(values, "ca"), "cert": stringOf(values, "cert"), "key": stringOf(values, "key"), "dh": stringOf(values, "dh"),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name": stringValue(section.Name), "enabled": boolValue(section.Options["enabled"] == "1"),
				"port": int64Value(parseOptionInt64(section.Options["port"])), "proto": stringValue(defaultString(section.Options["proto"], "udp")),
				"dev": stringValue(defaultString(section.Options["dev"], "tun")), "server": stringValue(section.Options["server"]),
				"ca": stringValue(section.Options["ca"]), "cert": stringValue(section.Options["cert"]), "key": stringValue(section.Options["key"]), "dh": stringValue(section.Options["dh"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be an OpenVPN section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
	})
}
