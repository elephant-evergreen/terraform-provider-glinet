package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewClientFilterListResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "client_filter_list",
		Description: "Manage the stock GL.iNet MAC filter address list. The stock init script stores entries as `mac` list values in the `black` or `white` section.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "List to manage: `black` or `white`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"macs": schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "MAC addresses in this list."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name != "black" && name != "white" {
				return router.Section{}, fmt.Errorf("name must be black or white")
			}
			return router.Section{Package: "gl-black_white_list", Type: "list", Name: name, Lists: map[string][]string{"mac": stringsOf(values, "macs")}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{"name": stringValue(section.Name), "macs": stringSet(section.Lists["mac"])}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "black" && id != "white" {
				return nil, fmt.Errorf("import ID must be black or white")
			}
			return map[string]attr.Value{"name": stringValue(id), "macs": stringSet(nil)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
	})
}

func NewNetworkIsolationResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "network_isolation",
		Description: "Manage the stock GL.iNet per-network isolation options (`secure_option` sections).",
		Attributes: map[string]schema.Attribute{
			"network":         schema.StringAttribute{Required: true, MarkdownDescription: "Network name, such as `lan`, `guest`, or `iot`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"ap_isolate":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Prevent wireless clients from reaching each other."},
			"wan_isolate":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Prevent this network from reaching the WAN."},
			"transfer_enable": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Allow transfers between isolated networks."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			network := stringOf(values, "network")
			if network == "" {
				return router.Section{}, fmt.Errorf("network is required")
			}
			return router.Section{Package: "gl-black_white_list", Type: "secure_option", Name: network, Options: map[string]string{
				"ap_isolate":      boolString(boolOf(values, "ap_isolate", false)),
				"wan_isolate":     boolString(boolOf(values, "wan_isolate", false)),
				"transfer_enable": boolString(boolOf(values, "transfer_enable", false)),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"network":         stringValue(section.Name),
				"ap_isolate":      boolValue(section.Options["ap_isolate"] == "1"),
				"wan_isolate":     boolValue(section.Options["wan_isolate"] == "1"),
				"transfer_enable": boolValue(section.Options["transfer_enable"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a network name")
			}
			return map[string]attr.Value{"network": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "network"), nil },
	})
}
