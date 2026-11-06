package provider

import (
	"context"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type systemDataSource struct{ client *router.Client }
type systemModel struct {
	Model    types.String `tfsdk:"model"`
	Firmware types.String `tfsdk:"firmware"`
	Mode     types.String `tfsdk:"mode"`
}

var _ datasource.DataSource = &systemDataSource{}
var _ datasource.DataSourceWithConfigure = &systemDataSource{}

func NewSystemDataSource() datasource.DataSource { return &systemDataSource{} }

func (d *systemDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system"
}

func (d *systemDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Read the verified router model, firmware version, and operating mode.", Attributes: map[string]schema.Attribute{
		"model":    schema.StringAttribute{Computed: true, MarkdownDescription: "Router model."},
		"firmware": schema.StringAttribute{Computed: true, MarkdownDescription: "Stock firmware version."},
		"mode":     schema.StringAttribute{Computed: true, MarkdownDescription: "GL.iNet operating mode."},
	}}
}

func (d *systemDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*router.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected a router client.")
		return
	}
	d.client = client
}

func (d *systemDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data systemModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	info, err := d.client.System(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read system information", err.Error())
		return
	}
	data.Model = types.StringValue(info.Model)
	data.Firmware = types.StringValue(info.Firmware)
	data.Mode = types.StringValue(info.Mode)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
