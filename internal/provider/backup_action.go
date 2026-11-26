package provider

import (
	"context"
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type backupAction struct{ client *router.Client }
type backupModel struct {
	Confirm types.String `tfsdk:"confirm"`
}

var (
	_ action.Action              = &backupAction{}
	_ action.ActionWithConfigure = &backupAction{}
)

func NewBackupAction() action.Action { return &backupAction{} }
func (a *backupAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup"
}
func (a *backupAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Create a verified encrypted router configuration backup. Requires `confirm = \"BACKUP\"`.",
		Attributes: map[string]schema.Attribute{"confirm": schema.StringAttribute{Required: true, MarkdownDescription: "Must be `BACKUP`."}}}
}
func (a *backupAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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
func (a *backupAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data backupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Confirm.ValueString() != "BACKUP" {
		resp.Diagnostics.AddError("Confirmation required", fmt.Sprintf("Set confirm = \"BACKUP\" to create a backup."))
		return
	}
	if a.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The router client is unavailable.")
		return
	}
	if err := a.client.CreateBackup(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot create backup", err.Error())
	}
}
