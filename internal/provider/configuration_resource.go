package provider

import (
	"context"
	"encoding/json"
	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type configurationResource struct{ client *router.Client }
type configurationModel struct {
	Configuration types.String `tfsdk:"configuration"`
	Keys          types.Map    `tfsdk:"wireless_keys_wo"`
	KeysVersion   types.Int64  `tfsdk:"wireless_keys_version"`
}

var (
	_ resource.Resource                   = &configurationResource{}
	_ resource.ResourceWithConfigure      = &configurationResource{}
	_ resource.ResourceWithValidateConfig = &configurationResource{}
)

func NewConfigurationResource() resource.Resource { return &configurationResource{} }
func (r *configurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_uci_configuration"
}
func (r *configurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manage a coordinated UCI configuration on one Flint 2 router. Adopts existing matching settings without mutation. Changed settings are committed together and protected by a router-side 300-second recovery timer that restarts after reboot. Network or wireless changes reboot the router. Destroy forgets management and preserves all settings. Use exactly one configuration resource per router and do not overlap with other resources or management tools.", Attributes: map[string]schema.Attribute{
		"configuration":         schema.StringAttribute{Required: true, MarkdownDescription: "JSON array of ordered UCI sections, preferably constructed with jsonencode. Each has package, name, type, optional scalar options and string lists. Supported packages: network, wireless, dhcp, firewall, gl-dns-v2, eth_ports_config_map. Only the supported nonsecret option allowlist is accepted. Named sections are created if missing. Anonymous selectors require exactly one section of the specified type, or a unique match map. Match maps are restricted to anonymous selectors. An explicit absent=true removes a named section; omitted sections and options remain untouched. Secret options are rejected. Configuration and network identifiers are stored in local Terraform state."},
		"wireless_keys_wo":      schema.MapAttribute{Optional: true, WriteOnly: true, Sensitive: true, ElementType: types.StringType, MarkdownDescription: "Write-only map from managed named wireless wifi-iface sections to WPA passphrases. Values are never stored in plan or state. Set wireless_keys_version and increase it to rotate keys. Omit this map to preserve existing keys. Key drift cannot be detected by refresh."},
		"wireless_keys_version": schema.Int64Attribute{Optional: true, MarkdownDescription: "Positive key rotation generation. Increase this value whenever wireless_keys_wo changes. Keys are applied only on creation or a generation change."},
	}}
}
func (r *configurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*router.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected a router client.")
		return
	}
	r.client = client
}
func (r *configurationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data configurationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.Configuration.IsUnknown() && !data.Configuration.IsNull() {
		if _, err := router.DecodeConfiguration(data.Configuration.ValueString()); err != nil {
			resp.Diagnostics.AddError("Invalid UCI configuration", err.Error())
		}
	}
	if !data.KeysVersion.IsUnknown() && !data.KeysVersion.IsNull() && data.KeysVersion.ValueInt64() < 1 {
		resp.Diagnostics.AddError("Invalid key generation", "wireless_keys_version must be a positive integer.")
	}
	if !data.Keys.IsNull() && data.KeysVersion.IsNull() {
		resp.Diagnostics.AddError("Missing key generation", "Set wireless_keys_version when using wireless_keys_wo.")
	}
}
func (r *configurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data configurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	config, err := router.DecodeConfiguration(data.Configuration.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid stored configuration", err.Error())
		return
	}
	actual, err := r.client.StableConfiguration(ctx, config)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read configuration", err.Error())
		return
	}
	for _, section := range actual {
		if section.Type == "" {
			resp.State.RemoveResource(ctx)
			return
		}
		for _, values := range section.Lists {
			if len(values) == 0 {
				resp.State.RemoveResource(ctx)
				return
			}
		}
	}
	if !router.SameConfiguration(actual, config) {
		encoded, _ := json.Marshal(actual)
		data.Configuration = types.StringValue(string(encoded))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *configurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data, config configurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	keys := map[string]string{}
	resp.Diagnostics.Append(config.Keys.ElementsAs(ctx, &keys, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, data, keys, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *configurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior, config configurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	keys := map[string]string{}
	if !data.KeysVersion.Equal(prior.KeysVersion) {
		resp.Diagnostics.Append(config.Keys.ElementsAs(ctx, &keys, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, data, keys, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}
func (r *configurationResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *configurationResource) write(ctx context.Context, data configurationModel, keys map[string]string, diagnostics *diag.Diagnostics) bool {
	config, err := router.DecodeConfiguration(data.Configuration.ValueString())
	if err == nil {
		err = r.client.WriteConfiguration(ctx, config, keys)
	}
	if err != nil {
		diagnostics.AddError("Cannot apply configuration", err.Error())
		return false
	}
	return true
}
