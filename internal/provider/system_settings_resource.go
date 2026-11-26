package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewSystemSettingsResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "system_settings",
		Description: "Manage the stock OpenWrt `system` identity settings. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"hostname": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Router hostname."},
			"timezone": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "IANA time zone, such as `Asia/Tokyo`."},
			"zonename": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Human-readable time zone name."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "system", Type: "system", Name: "@system[0]", Options: map[string]string{
				"hostname": stringOf(values, "hostname"), "timezone": stringOf(values, "timezone"), "zonename": stringOf(values, "zonename"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "system", Type: "system", Name: "@system[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"hostname": stringValue(section.Options["hostname"]),
				"timezone": stringValue(section.Options["timezone"]),
				"zonename": stringValue(section.Options["zonename"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "system_settings" {
				return nil, fmt.Errorf("import ID must be system_settings")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "system_settings", nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "hostname") == "" {
				return fmt.Errorf("hostname is required")
			}
			return nil
		},
	})
}
