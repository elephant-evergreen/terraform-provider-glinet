package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewSystemNTPResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "system_ntp",
		Description: "Manage the stock OpenWrt NTP client. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable NTP synchronization."},
			"servers":   schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "NTP servers."},
			"use_dhcp":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Use NTP servers provided by DHCP."},
			"interface": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional NTP interface."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "system", Type: "timeserver", Name: "ntp", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", true)), "use_dhcp": boolString(boolOf(values, "use_dhcp", false)), "interface": stringOf(values, "interface"),
			}, Lists: map[string][]string{"server": stringsOf(values, "servers")}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "system", Type: "timeserver", Name: "ntp"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enabled"] != "0"), "servers": stringSet(section.Lists["server"]),
				"use_dhcp": boolValue(section.Options["use_dhcp"] == "1"), "interface": stringValue(section.Options["interface"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "system_ntp" {
				return nil, fmt.Errorf("import ID must be system_ntp")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "system_ntp", nil },
	})
}
