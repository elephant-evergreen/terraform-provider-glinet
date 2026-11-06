package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewNetworkBridgeResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "network_bridge",
		Description: "Manage an OpenWrt bridge device and its port membership. The resource resolves a bridge by its `name` option, so stock bridges such as `br-lan` can be imported without knowing their anonymous UCI indices.",
		Attributes: map[string]schema.Attribute{
			"name":          schema.StringAttribute{Required: true, MarkdownDescription: "Bridge device name, such as `br-lan` or `br-vlan11`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"ports":         schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Bridge member ports. Omitted or empty membership removes all member ports, subject to management-path safety checks."},
			"empty":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Set `bridge_empty` so the bridge can exist without ports."},
			"stp":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable spanning tree protocol."},
			"igmp_snooping": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable IGMP snooping."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{"name": name, "type": "bridge", "bridge_empty": boolString(boolOf(values, "empty", false)), "stp": boolString(boolOf(values, "stp", false)), "igmp_snooping": boolString(boolOf(values, "igmp_snooping", false))}
			section := router.Section{Package: "network", Type: "device", Name: "@device[0]", Match: map[string]string{"name": name}, Options: options}
			ports := stringsOf(values, "ports")
			section.Lists = map[string][]string{"ports": ports}
			return section, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":          stringValue(section.Options["name"]),
				"ports":         stringSet(section.Lists["ports"]),
				"empty":         boolValue(section.Options["bridge_empty"] == "1"),
				"stp":           boolValue(section.Options["stp"] == "1"),
				"igmp_snooping": boolValue(section.Options["igmp_snooping"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a bridge name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			name := stringOf(values, "name")
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			return name, nil
		},
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" {
				return fmt.Errorf("name is required")
			}
			return nil
		},
	})
}
