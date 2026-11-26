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

func NewOpenVPNResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "openvpn",
		Description: "Manage the stock OpenVPN custom-config switch. The referenced OpenVPN profile file is not managed by this resource.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable the custom OpenVPN configuration."},
			"config":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("/etc/openvpn/my-vpn.conf"), MarkdownDescription: "OpenVPN configuration file path."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "openvpn", Type: "openvpn", Name: "custom_config", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "config": stringOf(values, "config"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "openvpn", Type: "openvpn", Name: "custom_config"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"enabled": boolValue(section.Options["enabled"] == "1"), "config": stringValue(section.Options["config"])}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "openvpn" {
				return nil, fmt.Errorf("import ID must be openvpn")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "openvpn", nil },
	})
}
