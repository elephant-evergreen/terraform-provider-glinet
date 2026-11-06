package provider

import (
	"context"
	"os"
	"strconv"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type glinetProvider struct{ version string }
type providerModel struct {
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	PrivateKey types.String `tfsdk:"private_key_file"`
	KnownHosts types.String `tfsdk:"known_hosts_file"`
}

var _ provider.Provider = &glinetProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &glinetProvider{version: version} }
}

func (p *glinetProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName, resp.Version = "glinet", p.version
}

func (p *glinetProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage independent network, wireless, DHCP, DNS, firewall and WireGuard objects on GL.iNet Flint 2 (GL-MT6000), stock firmware 4.11.0 in router mode over verified SSH.",
		Attributes: map[string]schema.Attribute{
			"host":             schema.StringAttribute{Optional: true, MarkdownDescription: "Router hostname or IP address. Falls back to `GLINET_HOST`."},
			"port":             schema.Int64Attribute{Optional: true, MarkdownDescription: "SSH port. Falls back to `GLINET_PORT`, then `22`."},
			"private_key_file": schema.StringAttribute{Optional: true, MarkdownDescription: "Path to an unencrypted OpenSSH private key for root. Falls back to `GLINET_PRIVATE_KEY_FILE`. Key contents are not part of the provider schema."},
			"known_hosts_file": schema.StringAttribute{Optional: true, MarkdownDescription: "Path to a known_hosts file containing the verified router host key. Falls back to `GLINET_KNOWN_HOSTS_FILE`. Host-key verification is always required."},
		},
	}
}

func (p *glinetProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	values := map[string]types.String{"host": config.Host, "private_key_file": config.PrivateKey, "known_hosts_file": config.KnownHosts}
	if config.Port.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("port"), "Unknown SSH port", "port must be known before apply. Set a static value or GLINET_PORT.")
	}
	for name, value := range values {
		if value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unknown connection setting", "Connection settings must be known before apply. Use a static value or the documented GLINET environment variable.")
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resolved := make(map[string]string)
	for name, value := range values {
		resolved[name] = value.ValueString()
		if value.IsNull() {
			resolved[name] = os.Getenv(map[string]string{"host": "GLINET_HOST", "port": "GLINET_PORT", "private_key_file": "GLINET_PRIVATE_KEY_FILE", "known_hosts_file": "GLINET_KNOWN_HOSTS_FILE"}[name])
		}
		if resolved[name] == "" {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Missing connection setting", "Set "+name+" or its documented GLINET environment variable. Explicit empty values are not replaced by environment variables.")
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	port := config.Port.ValueInt64()
	if config.Port.IsNull() {
		port = 22
		if value := os.Getenv("GLINET_PORT"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				resp.Diagnostics.AddAttributeError(path.Root("port"), "Invalid SSH port", "GLINET_PORT must be a decimal integer between 1 and 65535.")
				return
			}
			port = parsed
		}
	}
	if port < 1 || port > 65535 {
		resp.Diagnostics.AddAttributeError(path.Root("port"), "Invalid SSH port", "port must be a decimal integer between 1 and 65535.")
		return
	}
	transport, err := router.NewSSH(resolved["host"], int(port), resolved["private_key_file"], resolved["known_hosts_file"])
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure SSH", err.Error())
		return
	}
	client := router.NewClient(transport)
	if err := client.Verify(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot verify router", err.Error())
		return
	}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *glinetProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetworkInterfaceResource,
		NewNetworkBridgeResource,
		NewEthernetPortResource,
		NewWirelessInterfaceResource,
		NewDHCPPoolResource,
		NewDHCPReservationResource,
		NewDNSResource,
		NewDNSMasqResource,
		NewFirewallZoneResource,
		NewFirewallForwardingResource,
		NewFirewallRuleResource,
		NewFirewallRedirectResource,
		NewWireGuardInterfaceResource,
		NewWireGuardPeerResource,
	}
}

func (p *glinetProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewSystemDataSource}
}
