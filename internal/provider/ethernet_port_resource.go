package provider

import (
	"fmt"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func NewEthernetPortResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "ethernet_port",
		Description: "Manage one GL.iNet `eth_ports_config_map` port role and PVID.",
		Attributes: map[string]schema.Attribute{
			"port":      schema.StringAttribute{Required: true, MarkdownDescription: "Port name, such as `lan2`.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"mode":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("lan"), MarkdownDescription: "Port mode: `lan` or `wan`."},
			"vlan_mode": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("access"), MarkdownDescription: "VLAN mode, normally `access`."},
			"pvid":      schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1), MarkdownDescription: "Port VLAN ID."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			port := stringOf(values, "port")
			if port == "" {
				return router.Section{}, fmt.Errorf("port is required")
			}
			return router.Section{Package: "eth_ports_config_map", Type: "port", Name: port, Options: map[string]string{
				"mode": stringOf(values, "mode"), "vlan_mode": stringOf(values, "vlan_mode"), "pvid": fmt.Sprintf("%d", int64Of(values, "pvid")),
			}}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			pvid, _ := strconv.ParseInt(section.Options["pvid"], 10, 64)
			return map[string]attr.Value{
				"port":      stringValue(section.Name),
				"mode":      stringValue(defaultString(section.Options["mode"], "lan")),
				"vlan_mode": stringValue(defaultString(section.Options["vlan_mode"], "access")),
				"pvid":      int64Value(pvid),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id == "" {
				return nil, fmt.Errorf("import ID must be a port name")
			}
			return map[string]attr.Value{"port": stringValue(id)}, nil
		},
		ID: func(values map[string]attr.Value) (string, error) {
			port := stringOf(values, "port")
			if port == "" {
				return "", fmt.Errorf("port is required")
			}
			return port, nil
		},
		Validate: func(values map[string]attr.Value) error {
			if stringOf(values, "port") == "" {
				return fmt.Errorf("port is required")
			}
			if pvid := int64Of(values, "pvid"); pvid < 1 || pvid > 4094 {
				return fmt.Errorf("pvid must be between 1 and 4094")
			}
			return nil
		},
	})
}
