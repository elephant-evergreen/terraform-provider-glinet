package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewTimerResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "timer",
		Description: "Manage the stock GL.iNet timer global settings. Individual schedules are separate resources.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable scheduled actions."},
			"2g_func":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("turn_onoff"), MarkdownDescription: "2.4 GHz scheduled action."},
			"5g_func":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("turn_onoff"), MarkdownDescription: "5 GHz scheduled action."},
			"6g_func":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("turn_onoff"), MarkdownDescription: "6 GHz scheduled action."},
			"mld_func": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("turn_onoff"), MarkdownDescription: "MLD scheduled action."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "global", Name: "global", Options: map[string]string{
				"enable":   boolString(boolOf(values, "enabled", true)),
				"2G_func":  stringOf(values, "2g_func"),
				"5G_func":  stringOf(values, "5g_func"),
				"6G_func":  stringOf(values, "6g_func"),
				"MLD_func": stringOf(values, "mld_func"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":  boolValue(section.Options["enable"] != "0"),
				"2g_func":  stringValue(defaultString(section.Options["2G_func"], "turn_onoff")),
				"5g_func":  stringValue(defaultString(section.Options["5G_func"], "turn_onoff")),
				"6g_func":  stringValue(defaultString(section.Options["6G_func"], "turn_onoff")),
				"mld_func": stringValue(defaultString(section.Options["MLD_func"], "turn_onoff")),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "timer" {
				return nil, fmt.Errorf("import ID must be timer")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "timer", nil },
	})
}

func NewWiFiTimerResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "wifi_timer",
		Description: "Manage one stock GL.iNet Wi-Fi on/off schedule, such as the `2gwifi` or `5gwifi` timer section.",
		Attributes: map[string]schema.Attribute{
			"name":          schema.StringAttribute{Required: true, MarkdownDescription: "Timer section name, such as `2gwifi`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable this schedule."},
			"func":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("turn_onoff"), MarkdownDescription: "Scheduled action."},
			"band":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Radio band, such as `2G`, `5G`, or `6G`."},
			"guest":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Apply to the guest network."},
			"turn_on_hour":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("07"), MarkdownDescription: "Turn-on hour."},
			"turn_on_min":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("00"), MarkdownDescription: "Turn-on minute."},
			"turn_off_hour": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("22"), MarkdownDescription: "Turn-off hour."},
			"turn_off_min":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("00"), MarkdownDescription: "Turn-off minute."},
			"week":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("0 1 2 3 4 5 6"), MarkdownDescription: "Space-separated weekday numbers."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			return router.Section{Package: "gl_timer", Type: "timer", Name: name, Options: map[string]string{
				"enable":       boolString(boolOf(values, "enabled", false)),
				"func":         stringOf(values, "func"),
				"band":         stringOf(values, "band"),
				"guest":        boolString(boolOf(values, "guest", false)),
				"turnon_hour":  stringOf(values, "turn_on_hour"),
				"turnon_min":   stringOf(values, "turn_on_min"),
				"turnoff_hour": stringOf(values, "turn_off_hour"),
				"turnoff_min":  stringOf(values, "turn_off_min"),
				"week":         stringOf(values, "week"),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name":          stringValue(section.Name),
				"enabled":       boolValue(section.Options["enable"] == "1"),
				"func":          stringValue(defaultString(section.Options["func"], "turn_onoff")),
				"band":          stringValue(section.Options["band"]),
				"guest":         boolValue(section.Options["guest"] == "1"),
				"turn_on_hour":  stringValue(section.Options["turnon_hour"]),
				"turn_on_min":   stringValue(section.Options["turnon_min"]),
				"turn_off_hour": stringValue(section.Options["turnoff_hour"]),
				"turn_off_min":  stringValue(section.Options["turnoff_min"]),
				"week":          stringValue(section.Options["week"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a gl_timer timer section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
	})
}

func NewLEDTimerResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "led_timer",
		Description: "Manage the stock GL.iNet LED on/off schedule.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable the LED schedule."},
			"turn_on_hour":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("07"), MarkdownDescription: "Turn-on hour."},
			"turn_on_min":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("00"), MarkdownDescription: "Turn-on minute."},
			"turn_off_hour": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("22"), MarkdownDescription: "Turn-off hour."},
			"turn_off_min":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("00"), MarkdownDescription: "Turn-off minute."},
			"week":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("0 1 2 3 4 5 6"), MarkdownDescription: "Space-separated weekday numbers."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "timer", Name: "led", Options: map[string]string{
				"enable":       boolString(boolOf(values, "enabled", false)),
				"turnon_hour":  stringOf(values, "turn_on_hour"),
				"turnon_min":   stringOf(values, "turn_on_min"),
				"turnoff_hour": stringOf(values, "turn_off_hour"),
				"turnoff_min":  stringOf(values, "turn_off_min"),
				"week":         stringOf(values, "week"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "timer", Name: "led"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":       boolValue(section.Options["enable"] == "1"),
				"turn_on_hour":  stringValue(section.Options["turnon_hour"]),
				"turn_on_min":   stringValue(section.Options["turnon_min"]),
				"turn_off_hour": stringValue(section.Options["turnoff_hour"]),
				"turn_off_min":  stringValue(section.Options["turnoff_min"]),
				"week":          stringValue(section.Options["week"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "led_timer" {
				return nil, fmt.Errorf("import ID must be led_timer")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "led_timer", nil },
	})
}

func NewScheduledRebootResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "scheduled_reboot",
		Description: "Manage the stock GL.iNet scheduled reboot.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable the scheduled reboot."},
			"hour":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("03"), MarkdownDescription: "Reboot hour."},
			"min":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("00"), MarkdownDescription: "Reboot minute."},
			"week":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("0 1 2 3 4 5 6"), MarkdownDescription: "Space-separated weekday numbers."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "timer", Name: "reboot", Options: map[string]string{
				"enable": boolString(boolOf(values, "enabled", false)),
				"hour":   stringOf(values, "hour"),
				"min":    stringOf(values, "min"),
				"week":   stringOf(values, "week"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_timer", Type: "timer", Name: "reboot"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enable"] == "1"),
				"hour":    stringValue(section.Options["hour"]),
				"min":     stringValue(section.Options["min"]),
				"week":    stringValue(section.Options["week"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "scheduled_reboot" {
				return nil, fmt.Errorf("import ID must be scheduled_reboot")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "scheduled_reboot", nil },
	})
}
