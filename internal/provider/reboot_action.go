package provider

import (
	"context"
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type rebootAction struct{ client *router.Client }
type rebootModel struct {
	Confirm types.String `tfsdk:"confirm"`
}

var (
	_ action.Action              = &rebootAction{}
	_ action.ActionWithConfigure = &rebootAction{}
)

func NewRebootAction() action.Action { return &rebootAction{} }

func (a *rebootAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reboot"
}

func (a *rebootAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reboot the router. This is a one-shot action, not a managed resource. Requires `confirm = \"REBOOT\"`.",
		Attributes: map[string]schema.Attribute{
			"confirm": schema.StringAttribute{Required: true, MarkdownDescription: "Must be `REBOOT`."},
		},
	}
}

func (a *rebootAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*router.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected a router client.")
		return
	}
	a.client = client
}

func (a *rebootAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data rebootModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Confirm.ValueString() != "REBOOT" {
		resp.Diagnostics.AddError("Confirmation required", fmt.Sprintf("Set confirm = \"REBOOT\" to reboot the router."))
		return
	}
	if a.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The router client is unavailable.")
		return
	}
	if err := a.client.Reboot(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot reboot router", err.Error())
	}
}
