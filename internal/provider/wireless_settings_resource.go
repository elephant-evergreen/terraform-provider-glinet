package provider

import (
	"context"
	"errors"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type wirelessSettingsResource struct{ client *router.Client }
type wirelessSettingsModel struct {
	Section  types.String `tfsdk:"section"`
	SSID     types.String `tfsdk:"ssid"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Hidden   types.Bool   `tfsdk:"hidden"`
	Isolate  types.Bool   `tfsdk:"isolate"`
}

var (
	_ resource.Resource                   = &wirelessSettingsResource{}
	_ resource.ResourceWithConfigure      = &wirelessSettingsResource{}
	_ resource.ResourceWithImportState    = &wirelessSettingsResource{}
	_ resource.ResourceWithValidateConfig = &wirelessSettingsResource{}
)

func NewWirelessSettingsResource() resource.Resource { return &wirelessSettingsResource{} }

func (r *wirelessSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wireless_interface_settings"
}

func (r *wirelessSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SSID, disabled, hidden, and client-isolation settings on an existing named OpenWrt wifi-iface AP section. This resource never creates or deletes a Wi-Fi interface. Destroy releases Terraform management and leaves router settings unchanged. Changes reload Wi-Fi and can interrupt connections. Use a wired management connection. Passwords, encryption, radios, and network membership are outside this resource's scope.",
		Attributes: map[string]schema.Attribute{
			"section":  schema.StringAttribute{Required: true, MarkdownDescription: "Existing named UCI section in wireless, such as `wifi2g`. Anonymous index selectors are not supported. Import using this section name.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"ssid":     schema.StringAttribute{Required: true, MarkdownDescription: "Wi-Fi network name, 1–32 UTF-8 bytes without control characters."},
			"disabled": schema.BoolAttribute{Required: true, MarkdownDescription: "Whether the Wi-Fi interface is disabled."},
			"hidden":   schema.BoolAttribute{Required: true, MarkdownDescription: "Whether the SSID is hidden from ordinary Wi-Fi network lists."},
			"isolate":  schema.BoolAttribute{Required: true, MarkdownDescription: "Whether wireless clients on this interface are isolated from one another. This does not replace firewall isolation between networks."},
		},
	}
}

func (r *wirelessSettingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data wirelessSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.Section.IsUnknown() && !data.Section.IsNull() && !router.ValidSection(data.Section.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("section"), "Invalid UCI section", "Use 1–64 letters, digits, or underscores for a named wireless section.")
	}
	if !data.SSID.IsUnknown() && !data.SSID.IsNull() && !router.ValidSSID(data.SSID.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("ssid"), "Invalid SSID", "Use 1–32 UTF-8 bytes without control characters.")
	}
}

func (r *wirelessSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*router.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider configuration", "The resource requires a configured GL.iNet router client.")
		return
	}
	r.client = client
}

func (data wirelessSettingsModel) settings() router.Settings {
	return router.Settings{Section: data.Section.ValueString(), SSID: data.SSID.ValueString(), Disabled: data.Disabled.ValueBool(), Hidden: data.Hidden.ValueBool(), Isolate: data.Isolate.ValueBool()}
}

func (r *wirelessSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data wirelessSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Write(ctx, data.settings()); err != nil {
		resp.Diagnostics.AddError("Cannot apply Wi-Fi settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data wirelessSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	settings, err := r.client.ReadSettingsStable(ctx, data.Section.ValueString())
	if errors.Is(err, router.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read Wi-Fi settings", err.Error())
		return
	}
	data.SSID = types.StringValue(settings.SSID)
	data.Disabled = types.BoolValue(settings.Disabled)
	data.Hidden = types.BoolValue(settings.Hidden)
	data.Isolate = types.BoolValue(settings.Isolate)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data wirelessSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Write(ctx, data.settings()); err != nil {
		resp.Diagnostics.AddError("Cannot update Wi-Fi settings", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *wirelessSettingsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *wirelessSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !router.ValidSection(req.ID) {
		resp.Diagnostics.AddError("Invalid import section", "Import using an existing named wireless section, such as wifi2g.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("section"), req, resp)
}
