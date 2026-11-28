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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func NewNetworkRuleResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "network_rule",
		Description: "Manage one OpenWrt `network` policy routing rule. The UCI section name is the stable identity.",
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true, MarkdownDescription: "UCI rule section name.", Validators: []validator.String{nonEmptyStringValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"lookup":   schema.StringAttribute{Required: true, MarkdownDescription: "Routing table to consult."},
			"priority": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Rule priority; zero leaves it unset."},
			"src":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Source address or CIDR."},
			"dest":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Destination address or CIDR."},
			"in":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Input interface."},
			"out":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Output interface."},
			"family":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Address family: `ipv4`, `ipv6`, or empty."},
			"invert":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Invert the match."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"lookup": stringOf(values, "lookup"), "src": stringOf(values, "src"), "dest": stringOf(values, "dest"),
				"in": stringOf(values, "in"), "out": stringOf(values, "out"), "family": stringOf(values, "family"),
				"invert": boolString(boolOf(values, "invert", false)),
			}
			if priority := int64Of(values, "priority"); priority > 0 {
				options["priority"] = strconv.FormatInt(priority, 10)
			}
			return router.Section{Package: "network", Type: "rule", Name: name, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			priority, _ := strconv.ParseInt(section.Options["priority"], 10, 64)
			return map[string]attr.Value{
				"name": stringValue(section.Name), "lookup": stringValue(section.Options["lookup"]),
				"priority": int64Value(priority), "src": stringValue(section.Options["src"]), "dest": stringValue(section.Options["dest"]),
				"in": stringValue(section.Options["in"]), "out": stringValue(section.Options["out"]),
				"family": stringValue(section.Options["family"]), "invert": boolValue(section.Options["invert"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a network rule section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "lookup") == "" {
				return fmt.Errorf("lookup is required")
			}
			if int64Of(values, "priority") < 0 {
				return fmt.Errorf("priority must be non-negative")
			}
			switch stringOf(values, "family") {
			case "", "ipv4", "ipv6":
			default:
				return fmt.Errorf("family must be ipv4, ipv6, or empty")
			}
			return nil
		},
	})
}
