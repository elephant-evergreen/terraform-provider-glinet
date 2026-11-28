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

func NewSystemSSHResource() resource.Resource {
	return newTypedResource(typedSpec{
		TypeName:    "system_ssh",
		Description: "Manage the stock OpenWrt dropbear SSH service. Destroy releases ownership and preserves the firmware section.",
		NoDelete:    true,
		Attributes: map[string]schema.Attribute{
			"enabled":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable the SSH service. Disabling it is refused because it removes the provider's management path; use the router console."},
			"port":               schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(22), MarkdownDescription: "SSH port. It must match the provider's configured port. Moving the endpoint is refused; change it from the router console and update the provider configuration and known_hosts entry."},
			"password_auth":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Allow password authentication."},
			"root_password_auth": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Allow root password authentication."},
			"interface":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Restrict SSH to an interface; empty leaves it unrestricted."},
			"idle_timeout":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), MarkdownDescription: "Idle timeout in seconds; zero leaves it unset."},
		},
		Selector: func(values map[string]attr.Value) (router.Section, error) {
			options := map[string]string{
				"enable":           boolString(boolOf(values, "enabled", true)),
				"Port":             fmt.Sprintf("%d", int64Of(values, "port")),
				"PasswordAuth":     boolString(boolOf(values, "password_auth", true)),
				"RootPasswordAuth": boolString(boolOf(values, "root_password_auth", true)),
				"Interface":        stringOf(values, "interface"),
				"IdleTimeout":      fmt.Sprintf("%d", int64Of(values, "idle_timeout")),
			}
			return router.Section{Package: "dropbear", Type: "dropbear", Name: "@dropbear[0]", Options: options}, nil
		},
		ReadSelector: func(map[string]attr.Value) (router.Section, error) {
			return router.Section{Package: "dropbear", Type: "dropbear", Name: "@dropbear[0]"}, nil
		},
		Load: func(section router.Section) map[string]attr.Value {
			return map[string]attr.Value{
				"enabled":            boolValue(section.Options["enable"] != "0"),
				"port":               int64Value(parseOptionInt64(section.Options["Port"])),
				"password_auth":      boolValue(section.Options["PasswordAuth"] != "0"),
				"root_password_auth": boolValue(section.Options["RootPasswordAuth"] != "0"),
				"interface":          stringValue(section.Options["Interface"]),
				"idle_timeout":       int64Value(parseOptionInt64(section.Options["IdleTimeout"])),
			}
		},
		Import: func(id string) (map[string]attr.Value, error) {
			if id != "system_ssh" {
				return nil, fmt.Errorf("import ID must be system_ssh")
			}
			return map[string]attr.Value{}, nil
		},
		ID: func(map[string]attr.Value) (string, error) { return "system_ssh", nil },
		Validate: func(values map[string]attr.Value) error {
			port := int64Of(values, "port")
			if port < 1 || port > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
			return nil
		},
		AuthorizeWrite: sshEndpointWritable,
	})
}

// sshEndpointWritable refuses an SSH change the provider cannot verify. The
// management path must stay on the endpoint the provider is connected through,
// so moving or disabling SSH is a console operation, not a Terraform one.
func sshEndpointWritable(client *router.Client, values map[string]attr.Value) error {
	if !boolOf(values, "enabled", true) {
		return fmt.Errorf("disabling SSH would remove the provider's management path. Disable SSH from the router console instead.")
	}
	if client == nil {
		return nil
	}
	management := client.ManagementPort()
	if management == 0 {
		return nil
	}
	port := int64Of(values, "port")
	if port != int64(management) {
		return fmt.Errorf("SSH port %d does not match the provider's management port %d. Change the endpoint from the router console, then update the provider's port and known_hosts entry.", port, management)
	}
	return nil
}
