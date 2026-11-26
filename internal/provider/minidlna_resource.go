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

func NewMiniDLNAResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "minidlna",
		Description: "Manage the stock MiniDLNA media server. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable MiniDLNA."},
			"port":          schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(8200), MarkdownDescription: "HTTP port."},
			"interface":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("br-lan"), MarkdownDescription: "Listening interface."},
			"friendly_name": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("OpenWrt DLNA Server"), MarkdownDescription: "DLNA display name."},
			"media_dir":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("/mnt"), MarkdownDescription: "Media directory."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "minidlna", Type: "minidlna", Name: "config", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "port": fmt.Sprintf("%d", int64Of(values, "port")),
				"interface": stringOf(values, "interface"), "friendly_name": stringOf(values, "friendly_name"), "media_dir": stringOf(values, "media_dir"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "minidlna", Type: "minidlna", Name: "config"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enabled"] == "1"), "port": int64Value(parseOptionInt64(section.Options["port"])),
				"interface": stringValue(section.Options["interface"]), "friendly_name": stringValue(section.Options["friendly_name"]), "media_dir": stringValue(section.Options["media_dir"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "minidlna" {
				return nil, fmt.Errorf("import ID must be minidlna")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "minidlna", nil },
	})
}
