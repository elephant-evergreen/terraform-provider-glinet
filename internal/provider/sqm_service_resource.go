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

func NewSQMServiceResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "sqm_service",
		Description: "Manage the stock SQM service singleton. Queue sections are managed by glinet_sqm.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable SQM."},
			"autorate": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable automatic rate estimation."},
			"upload":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Global upload limit in kbit/s."},
			"download": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Global download limit in kbit/s."},
			"qdisc":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("cake"), MarkdownDescription: "Default queueing discipline."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "sqm", Type: "service", Name: "@service[0]", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)), "autorate": boolString(boolOf(values, "autorate", false)),
				"upload": fmt.Sprintf("%d", int64Of(values, "upload")), "download": fmt.Sprintf("%d", int64Of(values, "download")),
				"qdisc": stringOf(values, "qdisc"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "sqm", Type: "service", Name: "@service[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enabled"] == "1"), "autorate": boolValue(section.Options["autorate"] == "1"),
				"upload": int64Value(parseOptionInt64(section.Options["upload"])), "download": int64Value(parseOptionInt64(section.Options["download"])),
				"qdisc": stringValue(defaultString(section.Options["qdisc"], "cake")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "sqm_service" {
				return nil, fmt.Errorf("import ID must be sqm_service")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "sqm_service", nil },
	})
}
