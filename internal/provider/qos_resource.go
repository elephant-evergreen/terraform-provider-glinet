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

func NewQoSResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "qos",
		Description: "Manage the stock QoS general settings. Per-WAN QoS sections are not managed by this resource.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable QoS."},
			"model":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("cake"), MarkdownDescription: "QoS model."},
			"upload":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1000), MarkdownDescription: "Upload limit in kbit/s."},
			"download": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1000), MarkdownDescription: "Download limit in kbit/s."},
			"sum":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Sum all interface limits."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "qos", Type: "qos", Name: "general", Options: map[string]string{
				"enable": boolString(boolOf(values, "enabled", false)), "model": stringOf(values, "model"),
				"upload": fmt.Sprintf("%d", int64Of(values, "upload")), "download": fmt.Sprintf("%d", int64Of(values, "download")),
				"sum": boolString(boolOf(values, "sum", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "qos", Type: "qos", Name: "general"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enable"] == "1"), "model": stringValue(defaultString(section.Options["model"], "cake")),
				"upload": int64Value(parseOptionInt64(section.Options["upload"])), "download": int64Value(parseOptionInt64(section.Options["download"])),
				"sum": boolValue(section.Options["sum"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "qos" {
				return nil, fmt.Errorf("import ID must be qos")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "qos", nil },
	})
}
