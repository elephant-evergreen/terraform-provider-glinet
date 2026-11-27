package provider

import (
	"fmt"
	"strings"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewQoSInterfaceResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "qos_interface",
		Description: "Manage one per-WAN QoS section. The stock section type is selected by `kind`.",
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true, MarkdownDescription: "UCI QoS section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"kind":     schema.StringAttribute{Required: true, MarkdownDescription: "Section type: `wan`, `wwan`, `tethering`, or `modem`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable this QoS section."},
			"upload":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1000), MarkdownDescription: "Upload limit in kbit/s."},
			"download": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1000), MarkdownDescription: "Download limit in kbit/s."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name, kind := stringOf(values, "name"), stringOf(values, "kind")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			switch kind {
			case "wan", "wwan", "tethering", "modem":
			default:
				return router.Section{}, fmt.Errorf("kind must be wan, wwan, tethering, or modem")
			}
			return router.Section{Package: "qos", Type: kind, Name: name, Options: map[string]string{"enabled": boolString(boolOf(values, "enabled", false)), "upload": fmt.Sprintf("%d", int64Of(values, "upload")), "download": fmt.Sprintf("%d", int64Of(values, "download"))}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"name": stringValue(section.Name), "kind": stringValue(section.Type), "enabled": boolValue(section.Options["enabled"] == "1"), "upload": int64Value(parseOptionInt64(section.Options["upload"])), "download": int64Value(parseOptionInt64(section.Options["download"]))}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			parts := strings.SplitN(id, ":", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("import ID must be kind:section")
			}
			return map[string]attr.Value{"kind": stringValue(parts[0]), "name": stringValue(parts[1])}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			return stringOf(values, "kind") + ":" + stringOf(values, "name"), nil
		},
	})
}
