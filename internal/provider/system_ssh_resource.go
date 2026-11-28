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
			"enabled":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enable the SSH service."},
			"port":               schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(22), MarkdownDescription: "SSH port."},
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
		Advisory: sshManagementAdvisory,
	})
}

// sshManagementAdvisory warns when a planned SSH change may remove the
// management path before the provider can verify the new endpoint.
func sshManagementAdvisory(plan, state map[string]attr.Value, creating bool, managementPort int) string {
	if !boolOf(plan, "enabled", true) {
		return "This change disables SSH, so it removes the provider's management path. The provider applies it after the transaction commits and cannot verify or reverse it. Re-enable SSH from the router console, and apply this change on its own."
	}
	port := int64Of(plan, "port")
	changed := creating || int64Of(state, "port") != port
	if !changed || (managementPort != 0 && port == int64(managementPort)) {
		return ""
	}
	return fmt.Sprintf("SSH will listen on port %d after the transaction commits. The provider cannot verify or revert the new endpoint, so update the provider's port and known_hosts entry (or use the router console) before the next operation. Apply this change on its own; the remaining resources in the same apply may fail once the port changes.", port)
}
