package provider

import (
	"fmt"
	"strings"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewFirewallForwardingResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "firewall_forwarding",
		Description: "Manage one firewall forwarding between zones. Identity is `src:dest`; destroy releases ownership by default.",
		Attributes: map[string]schema.Attribute{
			"src":     schema.StringAttribute{Required: true, MarkdownDescription: "Source zone.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"dest":    schema.StringAttribute{Required: true, MarkdownDescription: "Destination zone.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the forwarding is active."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			src, dest := stringOf(values, "src"), stringOf(values, "dest")
			if src == "" || dest == "" {
				return router.Section{}, fmt.Errorf("src and dest are required")
			}
			return router.Section{Package: "firewall", Type: "forwarding", Name: "@forwarding[0]", Match: map[string]string{"src": src, "dest": dest}, Options: map[string]string{"src": src, "dest": dest, "enabled": boolString(boolOf(values, "enabled", true))}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"src":     stringValue(section.Options["src"]),
				"dest":    stringValue(section.Options["dest"]),
				"enabled": boolValue(section.Options["enabled"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			parts := strings.Split(id, ":")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("import ID must be src:dest")
			}
			return map[string]attr.Value{"src": stringValue(parts[0]), "dest": stringValue(parts[1])}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			src, dest := stringOf(values, "src"), stringOf(values, "dest")
			if src == "" || dest == "" {
				return "", fmt.Errorf("src and dest are required")
			}
			return src + ":" + dest, nil
		},
	})
}
