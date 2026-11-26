package provider

import (
	"fmt"
	"net"
	"regexp"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewDHCPReservationResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dhcp_reservation",
		Description: "Manage one OpenWrt `dhcp` host reservation. Static reservations may use an address inside the dynamic pool; UCI gives the reservation precedence.",
		Attributes: map[string]schema.Attribute{
			"section":   schema.StringAttribute{Required: true, MarkdownDescription: "UCI host section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"mac":       schema.StringAttribute{Required: true, MarkdownDescription: "Client MAC address."},
			"ip":        schema.StringAttribute{Required: true, MarkdownDescription: "Reserved IPv4 address."},
			"hostname":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "DHCP hostname."},
			"dns":       schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "Per-host DNS servers."},
			"duid":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "DHCPv6 DUID."},
			"leasetime": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Per-host lease time."},
			"ignore":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Ignore this host."},
			"instance":  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional dnsmasq instance name."},
			"tag":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional dnsmasq tag."},
			"set_tag":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Optional tag to set for this host."},
			"broadcast": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Force broadcast replies."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			section := stringOf(values, "section")
			if section == "" {
				return router.Section{}, fmt.Errorf("section is required")
			}
			options := map[string]string{"mac": stringOf(values, "mac"), "ip": stringOf(values, "ip")}
			name := stringOf(values, "hostname")
			options["name"] = name
			options["duid"] = stringOf(values, "duid")
			options["leasetime"] = stringOf(values, "leasetime")
			options["ignore"] = boolString(boolOf(values, "ignore", false))
			options["instance"] = stringOf(values, "instance")
			options["tag"] = stringOf(values, "tag")
			options["set_tag"] = stringOf(values, "set_tag")
			options["broadcast"] = boolString(boolOf(values, "broadcast", false))
			item := router.Section{Package: "dhcp", Type: "host", Name: section, Options: options}
			servers := stringsOf(values, "dns")
			item.Lists = map[string][]string{"dns": servers}
			return item, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"section":   stringValue(section.Name),
				"mac":       stringValue(section.Options["mac"]),
				"ip":        stringValue(section.Options["ip"]),
				"hostname":  stringValue(section.Options["name"]),
				"dns":       stringSet(section.Lists["dns"]),
				"duid":      stringValue(section.Options["duid"]),
				"leasetime": stringValue(section.Options["leasetime"]),
				"ignore":    boolValue(section.Options["ignore"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a host section name")
			}
			return map[string]attr.Value{"section": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) { return stringOf(values, "section"), nil },
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "section") == "" {
				return fmt.Errorf("section is required")
			}
			if _, err := net.ParseMAC(stringOf(values, "mac")); err != nil {
				return fmt.Errorf("mac is invalid")
			}
			ip := net.ParseIP(stringOf(values, "ip"))
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("ip must be IPv4")
			}
			if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]{0,62}$`).MatchString(defaultString(stringOf(values, "hostname"), "host")) {
				return fmt.Errorf("hostname is invalid")
			}
			return nil
		},
	})
}
