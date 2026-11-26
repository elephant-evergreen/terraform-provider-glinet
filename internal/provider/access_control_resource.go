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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewClientFilterResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "client_filter",
		Description: "Manage the stock GL.iNet client MAC filter mode. The address list itself is left to the stock UI.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable the client filter."},
			"mode":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("black"), MarkdownDescription: "Filter mode: `black` or `white`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl-black_white_list", Type: "global", Name: "global", Options: map[string]string{
				"enable": boolString(boolOf(values, "enabled", false)),
				"mode":   stringOf(values, "mode"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl-black_white_list", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enable"] == "1"),
				"mode":    stringValue(defaultString(section.Options["mode"], "black")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "client_filter" {
				return nil, fmt.Errorf("import ID must be client_filter")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "client_filter", nil },
		Validate: func(values map[string]attr.Value) error {
			if mode := stringOf(values, "mode"); mode != "black" && mode != "white" {
				return fmt.Errorf("mode must be black or white")
			}
			return nil
		},
	})
}

func NewWANAccessResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "wan_access",
		Description: "Manage the stock GL.iNet WAN access whitelist switch.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"whitelist": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Restrict WAN access to explicitly allowed clients."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "wan-access", Type: "main", Name: "@main[0]", Options: map[string]string{
				"whitelist": boolString(boolOf(values, "whitelist", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "wan-access", Type: "main", Name: "@main[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"whitelist": boolValue(section.Options["whitelist"] == "1")}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "wan_access" {
				return nil, fmt.Errorf("import ID must be wan_access")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "wan_access", nil },
	})
}

func NewParentalControlResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "parental_control",
		Description: "Manage the stock GL.iNet parental control global settings. Filter rules are left to the stock UI.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable parental control."},
			"drop_anonymous": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Drop traffic from unknown clients."},
			"auto_update":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Update the rule set automatically."},
			"log_level":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(2), MarkdownDescription: "Log level."},
			"source_devices": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Interfaces the rules apply to, such as `br-lan`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "parental_control_v2", Type: "global", Name: "global", Options: map[string]string{
				"enable":         boolString(boolOf(values, "enabled", false)),
				"drop_anonymous": boolString(boolOf(values, "drop_anonymous", false)),
				"auto_update":    boolString(boolOf(values, "auto_update", false)),
				"log_level":      strconv.FormatInt(int64Of(values, "log_level"), 10),
			}, Lists: map[string][]string{"src_dev": stringsOf(values, "source_devices")}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "parental_control_v2", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":        boolValue(section.Options["enable"] == "1"),
				"drop_anonymous": boolValue(section.Options["drop_anonymous"] == "1"),
				"auto_update":    boolValue(section.Options["auto_update"] == "1"),
				"log_level":      int64Value(parseOptionInt64(section.Options["log_level"])),
				"source_devices": stringSet(section.Lists["src_dev"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "parental_control" {
				return nil, fmt.Errorf("import ID must be parental_control")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "parental_control", nil },
		Validate: func(values map[string]attr.Value) error {
			if level := int64Of(values, "log_level"); level < 0 || level > 5 {
				return fmt.Errorf("log_level must be between 0 and 5")
			}
			return nil
		},
	})
}
