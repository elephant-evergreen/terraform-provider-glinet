package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewAdminAccessResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "admin_access",
		Description: "Manage the stock uhttpd admin access singleton. Certificate files are referenced, not managed.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Enable the web admin service."},
			"listen_http":     schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "HTTP listen addresses."},
			"listen_https":    schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), MarkdownDescription: "HTTPS listen addresses."},
			"redirect_https":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Redirect HTTP to HTTPS."},
			"rfc1918_filter":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Restrict access from private networks."},
			"max_requests":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3), MarkdownDescription: "Maximum concurrent requests per client."},
			"max_connections": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(100), MarkdownDescription: "Maximum connections."},
			"no_dirlists":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Disable directory listings."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "uhttpd", Type: "uhttpd", Name: "main", Options: map[string]string{
				"enabled":        boolString(boolOf(values, "enabled", false)),
				"redirect_https": boolString(boolOf(values, "redirect_https", false)),
				"rfc1918_filter": boolString(boolOf(values, "rfc1918_filter", true)), "max_requests": fmt.Sprintf("%d", int64Of(values, "max_requests")),
				"max_connections": fmt.Sprintf("%d", int64Of(values, "max_connections")), "no_dirlists": boolString(boolOf(values, "no_dirlists", true)),
			}, Lists: map[string][]string{"listen_http": stringsOf(values, "listen_http"), "listen_https": stringsOf(values, "listen_https")}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "uhttpd", Type: "uhttpd", Name: "main"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled": boolValue(section.Options["enabled"] == "1"), "listen_http": stringSet(section.Lists["listen_http"]),
				"listen_https": stringSet(section.Lists["listen_https"]), "redirect_https": boolValue(section.Options["redirect_https"] == "1"),
				"rfc1918_filter": boolValue(section.Options["rfc1918_filter"] != "0"), "max_requests": int64Value(parseOptionInt64(section.Options["max_requests"])),
				"max_connections": int64Value(parseOptionInt64(section.Options["max_connections"])), "no_dirlists": boolValue(section.Options["no_dirlists"] == "1"),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "admin_access" {
				return nil, fmt.Errorf("import ID must be admin_access")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "admin_access", nil },
	})
}
