package provider

import (
	"fmt"
	"strconv"

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

func NewRoutePolicyRuleResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "route_policy_rule",
		Description: "Manage one stock GL.iNet policy routing rule. `kind` selects the UCI section type written by the firmware.",
		Attributes: map[string]schema.Attribute{
			"name":      schema.StringAttribute{Required: true, MarkdownDescription: "UCI rule section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"kind":      schema.StringAttribute{Required: true, MarkdownDescription: "Rule type: `default` or `rule_process`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable this rule."},
			"via":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Tunnel or route the rule selects."},
			"from_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Match source, such as `device` or `process_gid`."},
			"from":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Match value for `from_type`."},
			"tunnel_id": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Tunnel identifier."},
			"mark":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Firewall mark, such as `0x8000`."},
			"label":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Rule label shown by the stock UI."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name, kind := stringOf(values, "name"), stringOf(values, "kind")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			switch kind {
			case "default", "rule_process":
			default:
				return router.Section{}, fmt.Errorf("kind must be default or rule_process")
			}
			options := map[string]string{
				"enabled":   boolString(boolOf(values, "enabled", true)),
				"from_type": stringOf(values, "from_type"),
				"from":      stringOf(values, "from"),
				"mark":      stringOf(values, "mark"),
				"name":      stringOf(values, "label"),
			}
			for _, field := range []struct{ attrName, option string }{{"via", "via"}} {
				options[field.option] = stringOf(values, field.attrName)
			}
			if tunnel := int64Of(values, "tunnel_id"); tunnel > 0 {
				options["tunnel_id"] = strconv.FormatInt(tunnel, 10)
			}
			if kind == "default" {
				options["type"] = "default"
			}
			return router.Section{Package: "route_policy", Type: kind, Name: name, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":      stringValue(section.Name),
				"kind":      stringValue(section.Type),
				"enabled":   boolValue(section.Options["enabled"] != "0"),
				"via":       stringValue(section.Options["via"]),
				"from_type": stringValue(section.Options["from_type"]),
				"from":      stringValue(section.Options["from"]),
				"tunnel_id": int64Value(parseOptionInt64(section.Options["tunnel_id"])),
				"mark":      stringValue(section.Options["mark"]),
				"label":     stringValue(section.Options["name"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			return nil, fmt.Errorf("import ID must be kind:section, such as default:gl_process")
		},
		ID: func(values map[string]attr.Value) (string, error) {
			return stringOf(values, "kind") + ":" + stringOf(values, "name"), nil
		},
	})
}
