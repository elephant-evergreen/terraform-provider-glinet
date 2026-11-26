package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
)

func NewDDNSResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "ddns",
		Description: "Manage the stock GL.iNet dynamic DNS service settings. Host and provider credentials live outside this section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled_ssh": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Allow SSH through DDNS."},
			"http_port":   schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(80), MarkdownDescription: "HTTP port published by DDNS."},
			"https_port":  schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(443), MarkdownDescription: "HTTPS port published by DDNS."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_ddns", Type: "service", Name: "ddns", Options: map[string]string{
				"enabled_ssh": boolString(boolOf(values, "enabled_ssh", false)),
				"http_port":   fmt.Sprintf("%d", int64Of(values, "http_port")),
				"https_port":  fmt.Sprintf("%d", int64Of(values, "https_port")),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl_ddns", Type: "service", Name: "ddns"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled_ssh": boolValue(section.Options["enabled_ssh"] == "1"),
				"http_port":   int64Value(parseOptionInt64(section.Options["http_port"])),
				"https_port":  int64Value(parseOptionInt64(section.Options["https_port"])),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "ddns" {
				return nil, fmt.Errorf("import ID must be ddns")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "ddns", nil },
		Validate: func(values map[string]attr.Value) error {
			for _, name := range []string{"http_port", "https_port"} {
				if port := int64Of(values, name); port < 1 || port > 65535 {
					return fmt.Errorf("%s must be between 1 and 65535", name)
				}
			}
			return nil
		},
	})
}
