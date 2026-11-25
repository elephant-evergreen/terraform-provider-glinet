package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewDNSResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dns",
		Description: "Manage the singleton GL.iNet stock DNS provider (`gl-dns-v2`). The encrypted-DNS provider metadata and endpoint are validated before apply. Destroy releases ownership and preserves the firmware section.",
		Attributes: map[string]schema.Attribute{
			"dns_provider":     schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("quad9"), MarkdownDescription: "Stock DNS provider name."},
			"protocol":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("doh"), MarkdownDescription: "Encrypted DNS protocol: `doh`, `dot`, or `udp`."},
			"mode":             schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("secure"), MarkdownDescription: "GL.iNet DNS mode."},
			"servers":          schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Provider server names from `/etc/dnsproxy/provider/<provider>`."},
			"manual_enable":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable manual DNS selection."},
			"manual_type":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("secure"), MarkdownDescription: "Manual DNS type."},
			"manual_local_dns": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Use the local encrypted DNS proxy."},
			"override_vpn":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Override VPN DNS."},
			"force_dns":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Force all clients through this DNS."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			servers := stringsOf(values, "servers")
			if len(servers) == 0 {
				return router.Section{}, fmt.Errorf("at least one DNS server is required")
			}
			return router.Section{
				Package: "gl-dns-v2", Type: "dns", Name: "@dns[0]",
				Options: map[string]string{
					"provider": stringOf(values, "dns_provider"), "proto": stringOf(values, "protocol"), "mode": stringOf(values, "mode"),
					"manual_enable": boolString(boolOf(values, "manual_enable", true)), "manual_type": stringOf(values, "manual_type"),
					"manual_local_dns": boolString(boolOf(values, "manual_local_dns", true)),
					"override_vpn":     boolString(boolOf(values, "override_vpn", false)), "force_dns": boolString(boolOf(values, "force_dns", false)),
				},
				Lists: map[string][]string{"servers_list": servers},
			}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "gl-dns-v2", Type: "dns", Name: "@dns[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"dns_provider":     stringValue(defaultString(section.Options["provider"], "quad9")),
				"protocol":         stringValue(defaultString(section.Options["proto"], "doh")),
				"mode":             stringValue(defaultString(section.Options["mode"], "secure")),
				"servers":          stringSet(section.Lists["servers_list"]),
				"manual_enable":    boolValue(section.Options["manual_enable"] != "0"),
				"manual_type":      stringValue(defaultString(section.Options["manual_type"], "secure")),
				"manual_local_dns": boolValue(section.Options["manual_local_dns"] != "0"),
				"override_vpn":     boolValue(section.Options["override_vpn"] == "1"),
				"force_dns":        boolValue(section.Options["force_dns"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "dns" {
				return nil, fmt.Errorf("import ID must be dns")
			}
			return map[string]attr.Value{"servers": types.SetValueMust(types.StringType, nil)}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "dns", nil },
		Validate: func(values map[string]attr.Value) error {
			if len(stringsOf(values, "servers")) == 0 {
				return fmt.Errorf("servers must contain at least one provider server")
			}
			protocol := stringOf(values, "protocol")
			if protocol != "doh" && protocol != "dot" && protocol != "udp" {
				return fmt.Errorf("protocol must be doh, dot, or udp")
			}
			return nil
		},
	})
}

func NewDNSMasqResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dnsmasq",
		Description: "Manage the singleton OpenWrt `dhcp` dnsmasq forwarding section. Destroy releases ownership and preserves the firmware section.",
		Attributes: map[string]schema.Attribute{
			"no_resolv":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Set `noresolv` so WAN-provided DNS is not used."},
			"servers":       schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "dnsmasq upstream servers, such as `127.0.0.1#5453`."},
			"domain":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional local domain."},
			"local":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional local service path."},
			"domain_needed": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Only forward fully qualified domain names."},
			"bogus_priv":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Reject private reverse DNS upstream requests."},
			"strictorder":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Respect the order of upstream servers."},
			"cachesize":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(150), MarkdownDescription: "DNS cache size."},
			"rebind_domain": schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Domains allowed to resolve to private addresses."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			servers := stringsOf(values, "servers")
			if len(servers) == 0 {
				return router.Section{}, fmt.Errorf("at least one upstream server is required")
			}
			options := map[string]string{"noresolv": boolString(boolOf(values, "no_resolv", true)),
				"domain_needed": boolString(boolOf(values, "domain_needed", false)),
				"bogus_priv":    boolString(boolOf(values, "bogus_priv", true)),
				"strictorder":   boolString(boolOf(values, "strictorder", false)),
				"cachesize":     fmt.Sprintf("%d", int64Of(values, "cachesize"))}
			domain := stringOf(values, "domain")
			options["domain"] = domain
			local := stringOf(values, "local")
			options["local"] = local
			return router.Section{Package: "dhcp", Type: "dnsmasq", Name: "@dnsmasq[0]", Options: options, Lists: map[string][]string{"server": servers, "rebind_domain": stringsOf(values, "rebind_domain")}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "dhcp", Type: "dnsmasq", Name: "@dnsmasq[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"no_resolv":     boolValue(section.Options["noresolv"] == "1"),
				"servers":       stringSet(section.Lists["server"]),
				"domain":        stringValue(section.Options["domain"]),
				"local":         stringValue(section.Options["local"]),
				"domain_needed": boolValue(section.Options["domain_needed"] == "1"),
				"bogus_priv":    boolValue(section.Options["bogus_priv"] != "0"),
				"strictorder":   boolValue(section.Options["strictorder"] == "1"),
				"cachesize":     int64Value(parseOptionInt64(section.Options["cachesize"])),
				"rebind_domain": stringSet(section.Lists["rebind_domain"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "dnsmasq" {
				return nil, fmt.Errorf("import ID must be dnsmasq")
			}
			return map[string]attr.Value{"servers": types.SetValueMust(types.StringType, nil)}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "dnsmasq", nil },
		Validate: func(values map[string]attr.Value) error {
			if len(stringsOf(values, "servers")) == 0 {
				return fmt.Errorf("servers must contain at least one upstream")
			}
			return nil
		},
	})
}
