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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewKMWANResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "kmwan",
		Description: "Manage the stock GL.iNet multi-WAN (kmwan) global settings. Members are managed by glinet_kmwan_member.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable multi-WAN."},
			"mode":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("failover"), MarkdownDescription: "Multi-WAN mode: `failover` or `loadbalance`."},
			"level":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("medium"), MarkdownDescription: "Tracking sensitivity level."},
			"sensitivity": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3000), MarkdownDescription: "Tracking sensitivity in milliseconds."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "kmwan", Type: "global", Name: "global", Options: map[string]string{
				"enable":      boolString(boolOf(values, "enabled", false)),
				"mode":        stringOf(values, "mode"),
				"level":       stringOf(values, "level"),
				"sensitivity": strconv.FormatInt(int64Of(values, "sensitivity"), 10),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "kmwan", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":     boolValue(section.Options["enable"] == "1"),
				"mode":        stringValue(defaultString(section.Options["mode"], "failover")),
				"level":       stringValue(defaultString(section.Options["level"], "medium")),
				"sensitivity": int64Value(parseOptionInt64(section.Options["sensitivity"])),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "kmwan" {
				return nil, fmt.Errorf("import ID must be kmwan")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "kmwan", nil },
		Validate: func(values map[string]attr.Value) error {
			if mode := stringOf(values, "mode"); mode != "failover" && mode != "loadbalance" {
				return fmt.Errorf("mode must be failover or loadbalance")
			}
			if int64Of(values, "sensitivity") < 0 {
				return fmt.Errorf("sensitivity must be non-negative")
			}
			return nil
		},
	})
}

func NewKMWANMemberResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "kmwan_member",
		Description: "Manage one GL.iNet multi-WAN member. The UCI section name is the stable identity.",
		Attributes: map[string]schema.Attribute{
			"name":       schema.StringAttribute{Required: true, MarkdownDescription: "UCI member section name, such as `wan` or `secondwan`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"interface":  schema.StringAttribute{Required: true, MarkdownDescription: "Logical interface tracked by this member."},
			"metric":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(10), MarkdownDescription: "Route metric."},
			"weight":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Load-balancing weight."},
			"track_mode": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("force"), MarkdownDescription: "Tracking mode."},
			"addr_type":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("4"), MarkdownDescription: "Address family: `4` or `6`."},
			"disabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Disable this member."},
			"check":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Track reachability."},
			"tracks":     schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Tracking probes, such as `ping,1.1.1.1`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{
				"interface":  stringOf(values, "interface"),
				"metric":     strconv.FormatInt(int64Of(values, "metric"), 10),
				"weight":     strconv.FormatInt(int64Of(values, "weight"), 10),
				"track_mode": stringOf(values, "track_mode"),
				"addr_type":  stringOf(values, "addr_type"),
				"disabled":   boolString(boolOf(values, "disabled", false)),
				"check":      boolString(boolOf(values, "check", true)),
			}
			return router.Section{Package: "kmwan", Type: "member", Name: name, Options: options, Lists: map[string][]string{"tracks": stringsOf(values, "tracks")}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":       stringValue(section.Name),
				"interface":  stringValue(section.Options["interface"]),
				"metric":     int64Value(parseOptionInt64(section.Options["metric"])),
				"weight":     int64Value(parseOptionInt64(section.Options["weight"])),
				"track_mode": stringValue(defaultString(section.Options["track_mode"], "force")),
				"addr_type":  stringValue(defaultString(section.Options["addr_type"], "4")),
				"disabled":   boolValue(section.Options["disabled"] == "1"),
				"check":      boolValue(section.Options["check"] != "0"),
				"tracks":     stringSet(section.Lists["tracks"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a kmwan member section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "interface") == "" {
				return fmt.Errorf("interface is required")
			}
			if family := stringOf(values, "addr_type"); family != "4" && family != "6" {
				return fmt.Errorf("addr_type must be 4 or 6")
			}
			if int64Of(values, "metric") < 0 || int64Of(values, "weight") < 1 {
				return fmt.Errorf("metric must be non-negative and weight at least 1")
			}
			return nil
		},
	})
}
