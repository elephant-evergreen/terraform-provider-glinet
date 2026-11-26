package provider

import (
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

func NewTailscaleResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "tailscale",
		Description: "Manage the stock Tailscale settings singleton. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"port":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(41641), MarkdownDescription: "Tailscale UDP port."},
			"log_stdout": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Log to stdout."},
			"log_stderr": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Log to stderr."},
			"state_file": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("/etc/tailscale/tailscaled.state"), MarkdownDescription: "Tailscale state file path."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "tailscale", Type: "settings", Name: "settings", Options: map[string]string{
				"port": fmt.Sprintf("%d", int64Of(values, "port")), "log_stdout": boolString(boolOf(values, "log_stdout", true)),
				"log_stderr": boolString(boolOf(values, "log_stderr", true)), "state_file": stringOf(values, "state_file"),
			}}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "tailscale", Type: "settings", Name: "settings"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"port": int64Value(parseOptionInt64(section.Options["port"])), "log_stdout": boolValue(section.Options["log_stdout"] == "1"),
				"log_stderr": boolValue(section.Options["log_stderr"] == "1"), "state_file": stringValue(section.Options["state_file"]),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "tailscale" {
				return nil, fmt.Errorf("import ID must be tailscale")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "tailscale", nil },
		Validate: func(values map[string]attr.Value) error {
			port := int64Of(values, "port")
			if port < 1 || port > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
			return nil
		},
	})
}
