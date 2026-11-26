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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewMeshResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "mesh",
		Description: "Manage the stock GL.iNet mesh base settings. Joining an agent to the mesh still requires the stock pairing flow.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable mesh networking."},
			"mode":               schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("controller"), MarkdownDescription: "Mesh role: `controller` or `agent`."},
			"log_level":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("DEBUG"), MarkdownDescription: "Mesh log level."},
			"log_path":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("/var/log/gl-mesh"), MarkdownDescription: "Mesh log path."},
			"onboard":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Onboarding state reported by the firmware."},
			"new_join_confirmed": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Whether a new join was confirmed."},
			"ts_enabled":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable the mesh guest/IoT VLAN split."},
			"ts_guest_vid":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(4002), MarkdownDescription: "Mesh guest VLAN ID."},
			"ts_iot_vid":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(4003), MarkdownDescription: "Mesh IoT VLAN ID."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl-mesh", Type: "base", Name: "@base[0]", Options: map[string]string{
				"enabled":            boolString(boolOf(values, "enabled", false)),
				"mode":               stringOf(values, "mode"),
				"log_level":          stringOf(values, "log_level"),
				"log_path":           stringOf(values, "log_path"),
				"onboard":            boolString(boolOf(values, "onboard", false)),
				"new_join_confirmed": boolString(boolOf(values, "new_join_confirmed", false)),
				"ts_enabled":         boolString(boolOf(values, "ts_enabled", true)),
				"ts_guest_vid":       strconv.FormatInt(int64Of(values, "ts_guest_vid"), 10),
				"ts_iot_vid":         strconv.FormatInt(int64Of(values, "ts_iot_vid"), 10),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl-mesh", Type: "base", Name: "@base[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":            boolValue(section.Options["enabled"] == "1"),
				"mode":               stringValue(defaultString(section.Options["mode"], "controller")),
				"log_level":          stringValue(defaultString(section.Options["log_level"], "DEBUG")),
				"log_path":           stringValue(defaultString(section.Options["log_path"], "/var/log/gl-mesh")),
				"onboard":            boolValue(section.Options["onboard"] == "1"),
				"new_join_confirmed": boolValue(section.Options["new_join_confirmed"] == "1"),
				"ts_enabled":         boolValue(section.Options["ts_enabled"] != "0"),
				"ts_guest_vid":       int64Value(parseOptionInt64(section.Options["ts_guest_vid"])),
				"ts_iot_vid":         int64Value(parseOptionInt64(section.Options["ts_iot_vid"])),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "mesh" {
				return nil, fmt.Errorf("import ID must be mesh")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "mesh", nil },
		Validate: func(values map[string]attr.Value) error {
			if mode := stringOf(values, "mode"); mode != "controller" && mode != "agent" {
				return fmt.Errorf("mode must be controller or agent")
			}
			for _, name := range []string{"ts_guest_vid", "ts_iot_vid"} {
				if vid := int64Of(values, name); vid < 1 || vid > 4094 {
					return fmt.Errorf("%s must be between 1 and 4094", name)
				}
			}
			return nil
		},
	})
}

func NewRepeaterResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "repeater",
		Description: "Manage the stock GL.iNet repeater settings. The upstream network itself is joined through the stock scan and connect flow.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"auto":              schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Automatically reconnect to a known network."},
			"level":             schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(6), MarkdownDescription: "Signal level threshold."},
			"switch_interval":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(30), MarkdownDescription: "Seconds between uplink switches."},
			"ip_wait_time":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(15), MarkdownDescription: "Seconds to wait for an address."},
			"blacklist_timeout": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(300), MarkdownDescription: "Seconds a failed network stays blacklisted."},
			"macaddr":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Repeater MAC address override."},
			"smart_reconnect":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Reconnect to the best known network."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "repeater", Type: "main", Name: "@main[0]", Options: map[string]string{
				"auto":              boolString(boolOf(values, "auto", true)),
				"level":             strconv.FormatInt(int64Of(values, "level"), 10),
				"switch_interval":   strconv.FormatInt(int64Of(values, "switch_interval"), 10),
				"ip_wait_time":      strconv.FormatInt(int64Of(values, "ip_wait_time"), 10),
				"blacklist_timeout": strconv.FormatInt(int64Of(values, "blacklist_timeout"), 10),
				"macaddr":           stringOf(values, "macaddr"),
				"smart_reconnect":   boolString(boolOf(values, "smart_reconnect", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "repeater", Type: "main", Name: "@main[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"auto":              boolValue(section.Options["auto"] != "0"),
				"level":             int64Value(parseOptionInt64(section.Options["level"])),
				"switch_interval":   int64Value(parseOptionInt64(section.Options["switch_interval"])),
				"ip_wait_time":      int64Value(parseOptionInt64(section.Options["ip_wait_time"])),
				"blacklist_timeout": int64Value(parseOptionInt64(section.Options["blacklist_timeout"])),
				"macaddr":           stringValue(section.Options["macaddr"]),
				"smart_reconnect":   boolValue(section.Options["smart_reconnect"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "repeater" {
				return nil, fmt.Errorf("import ID must be repeater")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "repeater", nil },
		Validate: func(values map[string]attr.Value) error {
			for _, name := range []string{"level", "switch_interval", "ip_wait_time", "blacklist_timeout"} {
				if int64Of(values, name) < 0 {
					return fmt.Errorf("%s must be non-negative", name)
				}
			}
			return nil
		},
	})
}
