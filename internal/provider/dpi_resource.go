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

func NewDPIResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dpi",
		Description: "Manage the stock GL.iNet DPI engine settings. Signature updates are handled by the firmware.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"log_level":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("INFO"), MarkdownDescription: "DPI log level."},
			"expire_time":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Flow expiry in seconds; zero leaves the firmware value."},
			"library_auto_update": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Update the DPI library automatically."},
			"ota_auto_update":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Update signatures over the air."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_dpi", Type: "gl_dpi", Name: "dpi_config", Options: map[string]string{
				"log":              stringOf(values, "log_level"),
				"expire_time":      strconv.FormatInt(int64Of(values, "expire_time"), 10),
				"lib_auto_upgrade": boolString(boolOf(values, "library_auto_update", false)),
				"ota_auto_upgrade": boolString(boolOf(values, "ota_auto_update", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_dpi", Type: "gl_dpi", Name: "dpi_config"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"log_level":           stringValue(defaultString(section.Options["log"], "INFO")),
				"expire_time":         int64Value(parseOptionInt64(section.Options["expire_time"])),
				"library_auto_update": boolValue(section.Options["lib_auto_upgrade"] == "1"),
				"ota_auto_update":     boolValue(section.Options["ota_auto_upgrade"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "dpi" {
				return nil, fmt.Errorf("import ID must be dpi")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "dpi", nil },
		Validate: func(values map[string]attr.Value) error {
			if int64Of(values, "expire_time") < 0 {
				return fmt.Errorf("expire_time must be non-negative")
			}
			return nil
		},
	})
}

func NewContentProtectionResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "content_protection",
		Description: "Manage the stock GL.iNet DPI content categories blocked on this router.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable content protection."},
			"categories": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Blocked categories, such as `gambling`, `adult`, or `malware`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_dpi_content_protection", Type: "dpi", Name: "content_protection", Options: map[string]string{
				"enabled": boolString(boolOf(values, "enabled", false)),
			}, Lists: map[string][]string{"category": stringsOf(values, "categories")}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_dpi_content_protection", Type: "dpi", Name: "content_protection"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":    boolValue(section.Options["enabled"] == "1"),
				"categories": stringSet(section.Lists["category"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "content_protection" {
				return nil, fmt.Errorf("import ID must be content_protection")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "content_protection", nil },
	})
}

func NewNATResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "nat",
		Description: "Manage the stock GL.iNet full-cone NAT switch.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"full_cone": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable full-cone NAT."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "fullconenat", Type: "global", Name: "global", Options: map[string]string{
				"enable": boolString(boolOf(values, "full_cone", false)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "fullconenat", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"full_cone": boolValue(section.Options["enable"] == "1")}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "nat" {
				return nil, fmt.Errorf("import ID must be nat")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "nat", nil },
	})
}

func NewHardwareAccelerationResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "hardware_acceleration",
		Description: "Manage the stock MediaTek hardware acceleration settings. Per-queue sections are not managed by this resource.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable hardware acceleration."},
			"hqos":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable hardware QoS."},
			"txq_num":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(16), MarkdownDescription: "Transmit queue count."},
			"scheduling": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("wrr"), MarkdownDescription: "Queue scheduling algorithm."},
			"acc_type":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("1"), MarkdownDescription: "Acceleration type reported by the firmware."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "mtkhnat", Type: "global", Name: "global", Options: map[string]string{
				"enable":     boolString(boolOf(values, "enabled", false)),
				"hqos":       boolString(boolOf(values, "hqos", false)),
				"txq_num":    strconv.FormatInt(int64Of(values, "txq_num"), 10),
				"scheduling": stringOf(values, "scheduling"),
				"actype":     stringOf(values, "acc_type"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "mtkhnat", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":    boolValue(section.Options["enable"] == "1"),
				"hqos":       boolValue(section.Options["hqos"] == "1"),
				"txq_num":    int64Value(parseOptionInt64(section.Options["txq_num"])),
				"scheduling": stringValue(defaultString(section.Options["scheduling"], "wrr")),
				"acc_type":   stringValue(defaultString(section.Options["actype"], "1")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "hardware_acceleration" {
				return nil, fmt.Errorf("import ID must be hardware_acceleration")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "hardware_acceleration", nil },
	})
}

func NewLEDResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "led",
		Description: "Manage the stock GL.iNet LED switch and daemon.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable the router LEDs."},
			"daemon":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Run the LED daemon."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_led", Type: "led", Name: "global", Options: map[string]string{
				"led_enable": boolString(boolOf(values, "enabled", true)),
				"led_daemon": boolString(boolOf(values, "daemon", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_led", Type: "led", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["led_enable"] != "0"),
				"daemon":  boolValue(section.Options["led_daemon"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "led" {
				return nil, fmt.Errorf("import ID must be led")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "led", nil },
	})
}
