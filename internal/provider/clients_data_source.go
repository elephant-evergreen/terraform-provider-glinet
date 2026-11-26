package provider

import (
	"context"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type clientsDataSource struct{ client *router.Client }

var _ datasource.DataSource = &clientsDataSource{}
var _ datasource.DataSourceWithConfigure = &clientsDataSource{}

func NewClientsDataSource() datasource.DataSource { return &clientsDataSource{} }

func (d *clientsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_clients"
}

func (d *clientsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read the DHCP lease table reported by the router. The lease table is runtime state, so the result changes without a configuration change.",
		Attributes: map[string]schema.Attribute{
			"clients": schema.ListAttribute{Computed: true, ElementType: clientsObjectType(), MarkdownDescription: "Active DHCP leases."},
		},
	}
}

func clientsObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"mac":           types.StringType,
		"ip":            types.StringType,
		"hostname":      types.StringType,
		"client_id":     types.StringType,
		"lease_seconds": types.Int64Type,
	}}
}

func (d *clientsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *clientsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var diagnostics = &resp.Diagnostics
	leases, err := d.client.DHCPLeases(ctx)
	if err != nil {
		diagnostics.AddError("Cannot read DHCP leases", err.Error())
		return
	}
	elements := make([]attr.Value, 0, len(leases))
	for _, lease := range leases {
		object, diags := types.ObjectValue(clientsObjectType().AttrTypes, map[string]attr.Value{
			"mac":           types.StringValue(lease.MAC),
			"ip":            types.StringValue(lease.IP),
			"hostname":      types.StringValue(lease.Hostname),
			"client_id":     types.StringValue(lease.ClientID),
			"lease_seconds": types.Int64Value(lease.LeaseSeconds),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		elements = append(elements, object)
	}
	result := map[string]attr.Value{"clients": types.ListValueMust(clientsObjectType(), elements)}
	state, diags := types.ObjectValue(map[string]attr.Type{"clients": types.ListType{ElemType: clientsObjectType()}}, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
