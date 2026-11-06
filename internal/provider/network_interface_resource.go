package provider

import (
	"fmt"
	"net"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewNetworkInterfaceResource() resource.Resource {
	return newTypedResource(typedNetworkInterfaceSpec())
}

func typedNetworkInterfaceSpec() typedSpec {
	return typedSpec{
		TypeName:    "network_interface",
		Description: "Manage one OpenWrt `network` interface section (including static addresses, VLAN interfaces, and DHCP clients). Destroy forgets ownership by default; set `delete_behavior = \"delete\"` to remove a named section.",
		Attributes: map[string]schema.Attribute{
			"name":         schema.StringAttribute{Required: true, MarkdownDescription: "UCI interface name, such as `lan`, `guest`, or `vlan11`. Changing it replaces the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"device":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Underlying device or bridge, such as `br-lan`."},
			"protocol":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("static"), MarkdownDescription: "UCI `proto`, normally `static`, `dhcp`, or `none`."},
			"address":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "IPv4 CIDR address, such as `192.168.8.1/24`."},
			"gateway":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional IPv4 gateway."},
			"dns":          schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "DNS servers for this interface."},
			"vlan_id":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "802.1Q VLAN ID for an interface. Zero means unset."},
			"display_name": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "GL.iNet display name."},
			"force_link":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Bring up the interface even without a carrier."},
			"delegate":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Delegate IPv6 prefix handling."},
			"enabled":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the interface is enabled."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{"proto": stringOf(values, "protocol"), "ipaddr": "", "netmask": "", "vlan_id": "", "force_link": boolString(boolOf(values, "force_link", false)), "delegate": boolString(boolOf(values, "delegate", false))}
			if options["proto"] == "" {
				options["proto"] = "static"
			}
			device := stringOf(values, "device")
			options["device"] = device
			if !boolOf(values, "enabled", true) {
				options["disabled"] = "1"
			} else {
				options["disabled"] = "0"
			}
			if address := stringOf(values, "address"); address != "" {
				ip, network, err := net.ParseCIDR(address)
				if err != nil || ip.To4() == nil {
					return router.Section{}, fmt.Errorf("address must be an IPv4 CIDR")
				}
				options["ipaddr"] = ip.String()
				options["netmask"] = net.IP(network.Mask).String()
			}
			gateway := stringOf(values, "gateway")
			options["gateway"] = gateway
			if vlan := int64Of(values, "vlan_id"); vlan > 0 {
				options["vlan_id"] = strconv.FormatInt(vlan, 10)
			}
			display := stringOf(values, "display_name")
			options["display_name"] = display

			section := router.Section{Package: "network", Type: "interface", Name: name, Options: options}
			dns := stringsOf(values, "dns")
			section.Lists = map[string][]string{"dns": dns}
			return section, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			address := ""
			if section.Options["ipaddr"] != "" {
				address = section.Options["ipaddr"]
				if section.Options["netmask"] != "" {
					ip := net.ParseIP(section.Options["ipaddr"])
					mask := net.ParseIP(section.Options["netmask"])
					if ip != nil && mask != nil {
						ones, bits := net.IPMask(mask.To4()).Size()
						if bits == 32 && ones >= 0 {
							address = ip.String() + "/" + strconv.Itoa(ones)
						}
					}
				}
			}
			vlan, _ := strconv.ParseInt(section.Options["vlan_id"], 10, 64)
			return map[string]attr.Value{
				"name":         stringValue(section.Name),
				"device":       stringValue(section.Options["device"]),
				"protocol":     stringValue(defaultString(section.Options["proto"], "static")),
				"address":      stringValue(address),
				"gateway":      stringValue(section.Options["gateway"]),
				"dns":          stringSet(section.Lists["dns"]),
				"vlan_id":      int64Value(vlan),
				"display_name": stringValue(section.Options["display_name"]),
				"force_link":   boolValue(section.Options["force_link"] == "1"),
				"delegate":     boolValue(section.Options["delegate"] == "1"),
				"enabled":      boolValue(section.Options["disabled"] != "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be an interface name")
			}
			return map[string]attr.Value{"name": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			name := stringOf(values, "name")
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			return name, nil
		},
		Validate: func(values map[string]attr.Value) error {
			address := stringOf(values, "address")
			protocol := stringOf(values, "protocol")
			if address != "" {
				ip, _, err := net.ParseCIDR(address)
				if err != nil || ip.To4() == nil {
					return fmt.Errorf("address must be an IPv4 CIDR")
				}
			}
			if protocol == "static" && address == "" {
				return fmt.Errorf("static interfaces require address")
			}
			if protocol == "dhcp" && address != "" {
				return fmt.Errorf("dhcp interfaces must not set address")
			}
			if vlan := int64Of(values, "vlan_id"); vlan < 0 || vlan > 4094 {
				return fmt.Errorf("vlan_id must be between 1 and 4094, or 0")
			}
			return nil
		},
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
