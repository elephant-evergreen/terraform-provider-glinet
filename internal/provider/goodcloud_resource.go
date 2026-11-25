package provider

import (
	"context"
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &goodCloudResource{}
	_ resource.ResourceWithConfigure   = &goodCloudResource{}
	_ resource.ResourceWithImportState = &goodCloudResource{}
)

type goodCloudClient interface {
	Cloud(context.Context) (router.CloudState, error)
	ApplyCloud(context.Context, bool) error
	ForgetCloud(context.Context) error
}

type goodCloudResource struct {
	client goodCloudClient
}

type goodCloudModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func NewGoodCloudResource() resource.Resource {
	return &goodCloudResource{}
}

func (r *goodCloudResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_goodcloud"
}

func (r *goodCloudResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(goodCloudClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected a GoodCloud client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *goodCloudResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage the local GoodCloud switch on GL.iNet Flint 2 (GL-MT6000). The resource calls the stock `cloud.set_config` operation, which stops the `rtty` service and may stop the `mptun` and `mpifd` services, may set `mptun.global.enable=0`, and may add a default `server` when none is configured. It does not bind or unbind the router in a GL.iNet account. Destroy releases provider ownership and leaves the current switch value unchanged.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Singleton identifier. Always `goodcloud`."},
			"enabled": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the stock GoodCloud agent is allowed to start. `false` calls `cloud.set_config` with `cloud_enable=false`, which also stops the stock `rtty` service, may stop `mptun` and `mpifd`, removes the local `bindtime`, may set `mptun.global.enable=0`, and may add a default `server` when none is configured. `true` starts the stock agent; a router without an account binding or server may not start a process, and Read reports that state as a warning. This resource does not change the GL.iNet account binding.",
			},
		},
	}
}

func (r *goodCloudResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan goodCloudModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Enabled.IsNull() || plan.Enabled.IsUnknown() {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Invalid GoodCloud configuration", "enabled must be known before apply.")
		}
		return
	}
	if err := r.client.ApplyCloud(ctx, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Cannot apply GoodCloud configuration", err.Error())
		return
	}
	plan.ID = types.StringValue("goodcloud")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *goodCloudResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state goodCloudModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	actual, err := r.client.Cloud(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read GoodCloud configuration", err.Error())
		return
	}
	state.ID = types.StringValue("goodcloud")
	state.Enabled = types.BoolValue(actual.Enabled)
	if !actual.Enabled && actual.Running {
		resp.Diagnostics.AddWarning(
			"GoodCloud is disabled but running",
			"The stock configuration has `cloud_enable=false`, but a `gl-cloud` process is running. The provider does not repair runtime state during refresh. Stop GoodCloud in the GL.iNet Admin Panel, or review and run `terraform apply -replace=<resource address>` (for example, `glinet_goodcloud.remote_management`).",
		)
	}
	if actual.Enabled && !actual.Running {
		resp.Diagnostics.AddWarning(
			"GoodCloud is enabled but not running",
			"The stock configuration has `cloud_enable=true`, but no `gl-cloud` process is running. The router may be unbound or missing a GoodCloud server. Verify the GoodCloud state in the GL.iNet Admin Panel.",
		)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *goodCloudResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan goodCloudModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Enabled.IsNull() || plan.Enabled.IsUnknown() {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Invalid GoodCloud configuration", "enabled must be known before apply.")
		}
		return
	}
	if err := r.client.ApplyCloud(ctx, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Cannot apply GoodCloud configuration", err.Error())
		return
	}
	plan.ID = types.StringValue("goodcloud")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *goodCloudResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state goodCloudModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.ForgetCloud(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot release GoodCloud ownership", err.Error())
	}
}

func (r *goodCloudResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "goodcloud" {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "Invalid import ID", "Use `goodcloud` as the import ID.")
		return
	}
	actual, err := r.client.Cloud(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read GoodCloud configuration", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, goodCloudModel{ID: types.StringValue("goodcloud"), Enabled: types.BoolValue(actual.Enabled)})...)
}
