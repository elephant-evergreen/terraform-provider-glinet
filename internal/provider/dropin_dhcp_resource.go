package provider

import (
	"fmt"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
)

func NewDropInDHCPResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "dropin_dhcp",
		Description: "Manage the stock GL.iNet drop-in gateway DHCP helper section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"lease_time": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(43200), MarkdownDescription: "Lease time in seconds."},
			"ignore":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Keep the helper pool from serving addresses."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "edgerouter", Type: "dhcp", Name: "wandhcp", Options: map[string]string{
				"leasetime": strconv.FormatInt(int64Of(values, "lease_time"), 10),
				"ignore":    boolString(boolOf(values, "ignore", true)),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "edgerouter", Type: "dhcp", Name: "wandhcp"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"lease_time": int64Value(parseOptionInt64(section.Options["leasetime"])),
				"ignore":     boolValue(section.Options["ignore"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "dropin_dhcp" {
				return nil, fmt.Errorf("import ID must be dropin_dhcp")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "dropin_dhcp", nil },
		Validate: func(values map[string]attr.Value) error {
			if int64Of(values, "lease_time") < 0 {
				return fmt.Errorf("lease_time must be non-negative")
			}
			return nil
		},
	})
}
