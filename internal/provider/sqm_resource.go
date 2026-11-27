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

func NewSQMResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "sqm",
		Description: "Manage one OpenWrt SQM queue section. The UCI section name is the stable identity.",
		Attributes: map[string]schema.Attribute{
			"name":           schema.StringAttribute{Required: true, MarkdownDescription: "UCI SQM queue section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"interface":      schema.StringAttribute{Required: true, MarkdownDescription: "Interface shaped by this queue."},
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable this queue."},
			"qdisc":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("cake"), MarkdownDescription: "Queueing discipline."},
			"upload":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Upload limit in kbit/s; zero leaves it unset."},
			"download":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Download limit in kbit/s; zero leaves it unset."},
			"script":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("piece_of_cake.qos"), MarkdownDescription: "SQM script."},
			"linklayer":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("none"), MarkdownDescription: "Link layer adaptation."},
			"overhead":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Per-packet overhead bytes."},
			"qdisc_advanced": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Use advanced qdisc options."},
			"ingress_ecn":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ECN"), MarkdownDescription: "Ingress ECN mode."},
			"egress_ecn":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("ECN"), MarkdownDescription: "Egress ECN mode."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"interface": stringOf(values, "interface"), "enabled": boolString(boolOf(values, "enabled", false)),
				"qdisc": stringOf(values, "qdisc"), "script": stringOf(values, "script"), "linklayer": stringOf(values, "linklayer"),
				"overhead": fmt.Sprintf("%d", int64Of(values, "overhead")), "qdisc_advanced": boolString(boolOf(values, "qdisc_advanced", false)),
				"ingress_ecn": stringOf(values, "ingress_ecn"), "egress_ecn": stringOf(values, "egress_ecn"),
			}
			if upload := int64Of(values, "upload"); upload > 0 {
				options["upload"] = fmt.Sprintf("%d", upload)
			}
			if download := int64Of(values, "download"); download > 0 {
				options["download"] = fmt.Sprintf("%d", download)
			}
			return router.Section{Package: "sqm", Type: "queue", Name: name, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name": stringValue(section.Name), "interface": stringValue(section.Options["interface"]),
				"enabled": boolValue(section.Options["enabled"] == "1"), "qdisc": stringValue(defaultString(section.Options["qdisc"], "cake")),
				"upload": int64Value(parseOptionInt64(section.Options["upload"])), "download": int64Value(parseOptionInt64(section.Options["download"])),
				"script": stringValue(defaultString(section.Options["script"], "piece_of_cake.qos")), "linklayer": stringValue(defaultString(section.Options["linklayer"], "none")),
				"overhead": int64Value(parseOptionInt64(section.Options["overhead"])), "qdisc_advanced": boolValue(section.Options["qdisc_advanced"] == "1"),
				"ingress_ecn": stringValue(defaultString(section.Options["ingress_ecn"], "ECN")), "egress_ecn": stringValue(defaultString(section.Options["egress_ecn"], "ECN")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be an SQM queue section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
	})
}
