package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func NewSystemLEDResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "system_led",
		Description: "Manage one OpenWrt `system` LED section. The UCI section name is the stable identity.",
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true, MarkdownDescription: "UCI LED section name.", Validators: []validator.String{nonEmptyStringValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"sysfs":    schema.StringAttribute{Required: true, MarkdownDescription: "LED sysfs device name."},
			"trigger":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("none"), MarkdownDescription: "LED trigger."},
			"dev":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Network device for the trigger."},
			"mode":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Trigger mode."},
			"default":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Default LED state."},
			"delayon":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Delay-on interval."},
			"delayoff": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Delay-off interval."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			return router.Section{Package: "system", Type: "led", Name: name, Options: map[string]string{
				"sysfs": stringOf(values, "sysfs"), "trigger": stringOf(values, "trigger"), "dev": stringOf(values, "dev"),
				"mode": stringOf(values, "mode"), "default": stringOf(values, "default"),
				"delayon": stringOf(values, "delayon"), "delayoff": stringOf(values, "delayoff"),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"name": stringValue(section.Name), "sysfs": stringValue(section.Options["sysfs"]),
				"trigger": stringValue(defaultString(section.Options["trigger"], "none")), "dev": stringValue(section.Options["dev"]),
				"mode": stringValue(section.Options["mode"]), "default": stringValue(section.Options["default"]),
				"delayon": stringValue(section.Options["delayon"]), "delayoff": stringValue(section.Options["delayoff"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be an LED section name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" || stringOf(values, "sysfs") == "" {
				return fmt.Errorf("name and sysfs are required")
			}
			return nil
		},
	})
}
