package provider

import (
	"fmt"
	"net"
	"strconv"
	"strings"

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

func NewWireGuardInterfaceResource() resource.Resource {
	return newTypedResource(typedWireGuardInterfaceSpec())
}

func typedWireGuardInterfaceSpec() typedSpec {
	return typedSpec{
		TypeName:    "wireguard_interface",
		Description: "Manage one OpenWrt WireGuard interface. The private key is a write-only argument; it is never stored in plan or state.",
		Attributes: map[string]schema.Attribute{
			"name":                schema.StringAttribute{Required: true, MarkdownDescription: "WireGuard interface name, such as `wg0`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"addresses":           schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Interface addresses in CIDR notation."},
			"listen_port":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(51820), MarkdownDescription: "UDP listen port."},
			"mtu":                 schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Optional MTU; zero leaves the firmware default."},
			"fwmark":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional firewall mark."},
			"no_host_route":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Do not add the host route."},
			"enabled":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the interface is enabled."},
			"private_key_wo":      schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Write-only WireGuard private key."},
			"private_key_version": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Client-managed key generation. Increase it when rotating `private_key_wo`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			name := stringOf(values, "name")
			if name == "" {
				return router.Section{}, fmt.Errorf("name is required")
			}
			options := map[string]string{"proto": "wireguard", "listen_port": strconv.FormatInt(int64Of(values, "listen_port"), 10), "disabled": boolString(!boolOf(values, "enabled", true))}
			if mtu := int64Of(values, "mtu"); mtu > 0 {
				options["mtu"] = strconv.FormatInt(mtu, 10)
			}
			if mark := stringOf(values, "fwmark"); mark != "" {
				options["fwmark"] = mark
			}
			if boolOf(values, "no_host_route", false) {
				options["nohostroute"] = "1"
			}
			section := router.Section{Package: "network", Type: "interface", Name: name, Options: options}
			if addresses := stringsOf(values, "addresses"); len(addresses) > 0 {
				section.Lists = map[string][]string{"addresses": addresses}
			}
			return section, nil
		},
		ReadSelector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "network", Type: "interface", Name: stringOf(values, "name")}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			port, _ := strconv.ParseInt(section.Options["listen_port"], 10, 64)
			mtu, _ := strconv.ParseInt(section.Options["mtu"], 10, 64)
			return map[string]attr.Value{
				"name": stringValue(section.Name), "addresses": stringSet(section.Lists["addresses"]),
				"listen_port": int64Value(port), "mtu": int64Value(mtu), "fwmark": stringValue(section.Options["fwmark"]),
				"no_host_route": boolValue(section.Options["nohostroute"] == "1"), "enabled": boolValue(section.Options["disabled"] != "1"),
				"private_key_wo": types.StringNull(),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a WireGuard interface name")
			}
			return map[string]attr.Value{"name": stringValue(id), "private_key_version": int64Value(1)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "name"), nil },
		Secrets: func(values map[string]attr.Value) map[string]string {
			key, name := stringOf(values, "private_key_wo"), stringOf(values, "name")
			if key == "" || name == "" {
				return nil
			}
			return map[string]string{"network." + name + ".private_key": key}
		},
		WriteOnly: []string{"private_key_wo"},
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "name") == "" {
				return fmt.Errorf("name is required")
			}
			if port := int64Of(values, "listen_port"); port < 1 || port > 65535 {
				return fmt.Errorf("listen_port must be between 1 and 65535")
			}
			for _, address := range stringsOf(values, "addresses") {
				if _, _, err := net.ParseCIDR(address); err != nil {
					return fmt.Errorf("addresses must be CIDRs")
				}
			}
			if key := stringOf(values, "private_key_wo"); key != "" && len(key) != 44 {
				return fmt.Errorf("private_key_wo must be a 44-character WireGuard key")
			}
			return nil
		},
	}
}

func NewWireGuardPeerResource() resource.Resource {
	return newTypedResource(typedWireGuardPeerSpec())
}

func typedWireGuardPeerSpec() typedSpec {
	return typedSpec{
		TypeName:    "wireguard_peer",
		Description: "Manage one WireGuard peer section on a managed interface. Preshared keys are write-only.",
		Attributes: map[string]schema.Attribute{
			"section":               schema.StringAttribute{Required: true, MarkdownDescription: "UCI wireguard peer section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"interface":             schema.StringAttribute{Required: true, MarkdownDescription: "WireGuard interface name, such as `wg0`."},
			"public_key":            schema.StringAttribute{Required: true, MarkdownDescription: "Peer public key."},
			"allowed_ips":           schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Allowed IP CIDRs."},
			"endpoint_host":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Peer endpoint host."},
			"endpoint_port":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Peer endpoint port; zero leaves it unset."},
			"persistent_keepalive":  schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Persistent keepalive seconds."},
			"route_allowed_ips":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Install routes for allowed IPs."},
			"description":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Peer description."},
			"preshared_key_wo":      schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Write-only WireGuard preshared key."},
			"preshared_key_version": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Client-managed key generation. Increase it when rotating `preshared_key_wo`."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section, interfaceName := stringOf(values, "section"), stringOf(values, "interface")
			if section == "" || interfaceName == "" {
				return router.Section{}, fmt.Errorf("section and interface are required")
			}
			options := map[string]string{"public_key": stringOf(values, "public_key")}
			if host := stringOf(values, "endpoint_host"); host != "" {
				options["endpoint_host"] = host
			}
			if port := int64Of(values, "endpoint_port"); port > 0 {
				options["endpoint_port"] = strconv.FormatInt(port, 10)
			}
			if keepalive := int64Of(values, "persistent_keepalive"); keepalive > 0 {
				options["persistent_keepalive"] = strconv.FormatInt(keepalive, 10)
			}
			if !boolOf(values, "route_allowed_ips", true) {
				options["route_allowed_ips"] = "0"
			}
			if description := stringOf(values, "description"); description != "" {
				options["description"] = description
			}
			item := router.Section{Package: "network", Type: "wireguard_" + interfaceName, Name: section, Options: options}
			if allowed := stringsOf(values, "allowed_ips"); len(allowed) > 0 {
				item.Lists = map[string][]string{"allowed_ips": allowed}
			}
			return item, nil
		},
		ReadSelector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "network", Type: "wireguard_" + stringOf(values, "interface"), Name: stringOf(values, "section")}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			port, _ := strconv.ParseInt(section.Options["endpoint_port"], 10, 64)
			keepalive, _ := strconv.ParseInt(section.Options["persistent_keepalive"], 10, 64)
			return map[string]attr.Value{
				"section": stringValue(section.Name), "interface": stringValue(strings.TrimPrefix(section.Type, "wireguard_")),
				"public_key": stringValue(section.Options["public_key"]), "allowed_ips": stringSet(section.Lists["allowed_ips"]),
				"endpoint_host": stringValue(section.Options["endpoint_host"]), "endpoint_port": int64Value(port),
				"persistent_keepalive": int64Value(keepalive), "route_allowed_ips": boolValue(section.Options["route_allowed_ips"] != "0"),
				"description": stringValue(section.Options["description"]), "preshared_key_wo": types.StringNull(),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			parts := strings.SplitN(id, ":", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("import ID must be interface:peer-section")
			}
			return map[string]attr.Value{"interface": stringValue(parts[0]), "section": stringValue(parts[1]), "preshared_key_version": int64Value(1)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			iface, section := stringOf(values, "interface"), stringOf(values, "section")
			if iface == "" || section == "" {
				return "", fmt.Errorf("interface and section are required")
			}
			return iface + ":" + section, nil
		},
		Secrets: func(values map[string]attr.Value) map[string]string {
			key, section := stringOf(values, "preshared_key_wo"), stringOf(values, "section")
			if key == "" || section == "" {
				return nil
			}
			return map[string]string{"network." + section + ".preshared_key": key}
		},
		WriteOnly: []string{"preshared_key_wo"},
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "section") == "" || stringOf(values, "interface") == "" || stringOf(values, "public_key") == "" {
				return fmt.Errorf("section, interface, and public_key are required")
			}
			for _, allowed := range stringsOf(values, "allowed_ips") {
				if _, _, err := net.ParseCIDR(allowed); err != nil {
					return fmt.Errorf("allowed_ips must be CIDRs")
				}
			}
			if key := stringOf(values, "preshared_key_wo"); key != "" && len(key) != 44 {
				return fmt.Errorf("preshared_key_wo must be a 44-character WireGuard key")
			}
			return nil
		},
	}
}
