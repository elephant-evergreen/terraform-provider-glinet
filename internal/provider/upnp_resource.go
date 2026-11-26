package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewUPnPResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "upnp",
		Description: "Manage the stock OpenWrt UPnP service. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable UPnP."},
			"internal_iface": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Internal interface."},
			"external_iface": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "External interface."},
			"secure_mode":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Restrict mappings to the requesting client."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "upnpd", Type: "upnpd", Name: "@upnpd[0]", Options: map[string]string{
				"enabled":        boolString(boolOf(values, "enabled", false)),
				"internal_iface": stringOf(values, "internal_iface"),
				"external_iface": stringOf(values, "external_iface"),
				"secure_mode":    boolString(boolOf(values, "secure_mode", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "upnpd", Type: "upnpd", Name: "@upnpd[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":        boolValue(section.Options["enabled"] == "1"),
				"internal_iface": stringValue(section.Options["internal_iface"]),
				"external_iface": stringValue(section.Options["external_iface"]),
				"secure_mode":    boolValue(section.Options["secure_mode"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "upnp" {
				return nil, fmt.Errorf("import ID must be upnp")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "upnp", nil },
	})
}
