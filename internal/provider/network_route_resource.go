package provider

import (
	"fmt"
	"net"
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

func NewNetworkRouteResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "network_route",
		Description: "Manage one OpenWrt `network` static route section. The UCI section name is the stable identity; stock routes can be imported by name.",
		Attributes: map[string]schema.Attribute{
			"name":      schema.StringAttribute{Required: true, MarkdownDescription: "UCI route section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"interface": schema.StringAttribute{Required: true, MarkdownDescription: "Interface used to reach the target."},
			"target":    schema.StringAttribute{Required: true, MarkdownDescription: "Target address or CIDR, such as `10.20.0.0/16`."},
			"gateway":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional gateway address."},
			"metric":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Route metric; zero leaves it unset."},
			"table":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional routing table."},
			"source":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional source address or CIDR."},
			"onlink":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Treat the gateway as directly reachable."},
			"mtu":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Optional route MTU; zero leaves it unset."},
			"type":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("unicast"), MarkdownDescription: "Route type, normally `unicast`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"interface": stringOf(values, "interface"),
				"target":    stringOf(values, "target"),
				"gateway":   stringOf(values, "gateway"),
				"metric":    strconv.FormatInt(int64Of(values, "metric"), 10),
				"table":     stringOf(values, "table"),
				"source":    stringOf(values, "source"),
				"onlink":    boolString(boolOf(values, "onlink", false)),
				"mtu":       strconv.FormatInt(int64Of(values, "mtu"), 10),
				"type":      stringOf(values, "type"),
			}
			return router.Section{Package: "network", Type: "route", Name: name, Options: options}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			metric, _ := strconv.ParseInt(section.Options["metric"], 10, 64)
			mtu, _ := strconv.ParseInt(section.Options["mtu"], 10, 64)
			return map[string]attr.Value{
				"name":      stringValue(section.Name),
				"interface": stringValue(section.Options["interface"]),
				"target":    stringValue(section.Options["target"]),
				"gateway":   stringValue(section.Options["gateway"]),
				"metric":    int64Value(metric),
				"table":     stringValue(section.Options["table"]),
				"source":    stringValue(section.Options["source"]),
				"onlink":    boolValue(section.Options["onlink"] == "1"),
				"mtu":       int64Value(mtu),
				"type":      stringValue(defaultString(section.Options["type"], "unicast")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a network route section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "interface") == "" {
				return fmt.Errorf("interface is required")
			}
			target := stringOf(values, "target")
			if _, _, err := net.ParseCIDR(target); err != nil {
				return fmt.Errorf("target must be an address or CIDR: %w", err)
			}
			if gateway := stringOf(values, "gateway"); gateway != "" && net.ParseIP(gateway) == nil {
				return fmt.Errorf("gateway must be an IP address")
			}
			if int64Of(values, "metric") < 0 || int64Of(values, "mtu") < 0 {
				return fmt.Errorf("metric and mtu must be non-negative")
			}
			return nil
		},
	})
}
