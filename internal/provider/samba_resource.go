package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewSambaResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "samba",
		Description: "Manage the stock GL.iNet Samba global settings. Share sections and account passwords are not managed by this resource.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"workgroup":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("WORKGROUP"), MarkdownDescription: "SMB workgroup."},
			"charset":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("UTF-8"), MarkdownDescription: "Unix character set."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("Samba on OpenWRT"), MarkdownDescription: "Server description."},
			"name":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "NetBIOS name; empty leaves the firmware value."},
			"interfaces":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Serving interfaces; empty leaves the firmware value."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "samba4", Type: "samba", Name: "@samba[0]", Options: map[string]string{
				"workgroup":   stringOf(values, "workgroup"),
				"charset":     stringOf(values, "charset"),
				"description": stringOf(values, "description"),
				"name":        stringOf(values, "name"),
				"interfaces":  stringOf(values, "interfaces"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "samba4", Type: "samba", Name: "@samba[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"workgroup":   stringValue(defaultString(section.Options["workgroup"], "WORKGROUP")),
				"charset":     stringValue(defaultString(section.Options["charset"], "UTF-8")),
				"description": stringValue(section.Options["description"]),
				"name":        stringValue(section.Options["name"]),
				"interfaces":  stringValue(section.Options["interfaces"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "samba" {
				return nil, fmt.Errorf("import ID must be samba")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "samba", nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "workgroup") == "" {
				return fmt.Errorf("workgroup is required")
			}
			return nil
		},
	})
}
