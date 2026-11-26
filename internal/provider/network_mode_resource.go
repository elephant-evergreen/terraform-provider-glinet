package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewNetworkModeResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "network_mode",
		Description: "Manage the stock GL.iNet operating mode and connectivity tracking. Changing the mode restarts networking.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"mode":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("router"), MarkdownDescription: "Operating mode, such as `router` or `ap`."},
			"autotimezone": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Derive the time zone from the uplink."},
			"track_ip":     schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "IPv4 reachability probes."},
			"track_ipv6":   schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "IPv6 reachability probes."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "glconfig", Type: "service", Name: "general", Options: map[string]string{
				"mode":         stringOf(values, "mode"),
				"autotimezone": boolString(boolOf(values, "autotimezone", true)),
			}, Lists: map[string][]string{
				"track_ip":   stringsOf(values, "track_ip"),
				"track_ipv6": stringsOf(values, "track_ipv6"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "glconfig", Type: "service", Name: "general"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"mode":         stringValue(defaultString(section.Options["mode"], "router")),
				"autotimezone": boolValue(section.Options["autotimezone"] != "0"),
				"track_ip":     stringSet(section.Lists["track_ip"]),
				"track_ipv6":   stringSet(section.Lists["track_ipv6"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "network_mode" {
				return nil, fmt.Errorf("import ID must be network_mode")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "network_mode", nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "mode") == "" {
				return fmt.Errorf("mode is required")
			}
			return nil
		},
	})
}

func NewDropInGatewayResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dropin_gateway",
		Description: "Manage the stock GL.iNet drop-in gateway switch. The helper DHCP section is left to the firmware.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable drop-in gateway mode."},
			"force_dns": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Force clients to use the router DNS."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "edgerouter", Type: "global", Name: "global", Options: map[string]string{
				"enabled":   boolString(boolOf(values, "enabled", false)),
				"force_dns": boolString(boolOf(values, "force_dns", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "edgerouter", Type: "global", Name: "global"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":   boolValue(section.Options["enabled"] == "1"),
				"force_dns": boolValue(section.Options["force_dns"] != "0"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "dropin_gateway" {
				return nil, fmt.Errorf("import ID must be dropin_gateway")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "dropin_gateway", nil },
	})
}
