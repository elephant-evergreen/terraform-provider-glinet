package provider

import (
	"context"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type typedSpec struct {
	TypeName     string
	Description  string
	Attributes   map[string]schema.Attribute
	Selector     func(map[string]attr.Value) (router.Section, error)
	ReadSelector func(map[string]attr.Value) (router.Section, error)
	Load         func(router.Section) map[string]attr.Value
	Import       func(string) (map[string]attr.Value, error)
	ID           func(map[string]attr.Value) (string, error)
	Validate     func(map[string]attr.Value) error
	Secrets      func(map[string]attr.Value) map[string]string
	WriteOnly    []string
}

type typedResource struct {
	client    *router.Client
	spec      typedSpec
	attrTypes map[string]attr.Type
	schema    schema.Schema
}

var (
	_ resource.Resource                = &typedResource{}
	_ resource.ResourceWithConfigure   = &typedResource{}
	_ resource.ResourceWithImportState = &typedResource{}
)

func newTypedResource(spec typedSpec) resource.Resource {
	attributes := map[string]schema.Attribute{}
	for name, attribute := range spec.Attributes {
		attributes[name] = attribute
	}
	attributes["id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Stable import identifier."}
	attributes["uci_section"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Resolved UCI section name on the router."}
	attributes["delete_behavior"] = schema.StringAttribute{
		Optional: true, Computed: true, Default: stringdefault.StaticString("forget"),
		MarkdownDescription: "Destroy behavior. `forget` releases Terraform ownership and preserves the UCI section; `delete` removes a Terraform-created named section. Firmware-owned anonymous sections are always preserved.",
		Validators:          []validator.String{deleteBehaviorValidator{}},
	}
	attrTypes := map[string]attr.Type{
		"id":              types.StringType,
		"uci_section":     types.StringType,
		"delete_behavior": types.StringType,
	}
	for name, attribute := range attributes {
		if name == "id" || name == "uci_section" || name == "delete_behavior" {
			continue
		}
		attrTypes[name] = attribute.GetType()
	}
	return &typedResource{
		spec:      spec,
		attrTypes: attrTypes,
		schema: schema.Schema{
			MarkdownDescription: spec.Description,
			Attributes:          attributes,
		},
	}
}

func (r *typedResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.TypeName
}

func (r *typedResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schema
}

func (r *typedResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *typedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var object types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &object)...)
	if resp.Diagnostics.HasError() || object.IsNull() || object.IsUnknown() {
		return
	}
	values := object.Attributes()
	if err := r.validate(values); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	section, err := r.spec.Selector(values)
	if err != nil {
		resp.Diagnostics.AddError("Cannot build section", err.Error())
		return
	}
	if section.Options == nil {
		section.Options = map[string]string{}
	}
	section.Options[router.ManagedOption] = "1"
	if err := r.client.ApplySections(ctx, router.Configuration{section}, nil, r.secrets(values)); err != nil {
		resp.Diagnostics.AddError("Cannot apply section", err.Error())
		return
	}
	object, diagnostics := r.stateObject(values, section)
	resp.Diagnostics.Append(diagnostics...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, object)...)
	}
}

func (r *typedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var object types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &object)...)
	if resp.Diagnostics.HasError() || object.IsNull() {
		return
	}
	values := object.Attributes()
	selector := r.spec.Selector
	if r.spec.ReadSelector != nil {
		selector = r.spec.ReadSelector
	}
	section, err := selector(values)
	if err != nil {
		resp.Diagnostics.AddError("Cannot build section selector", err.Error())
		return
	}
	actual, found, err := r.client.ReadSection(ctx, section)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read section", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	object, diagnostics := r.stateObject(values, actual)
	resp.Diagnostics.Append(diagnostics...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, object)...)
	}
}

func (r *typedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var object types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &object)...)
	if resp.Diagnostics.HasError() || object.IsNull() || object.IsUnknown() {
		return
	}
	values := object.Attributes()
	if err := r.validate(values); err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	section, err := r.spec.Selector(values)
	if err != nil {
		resp.Diagnostics.AddError("Cannot build section", err.Error())
		return
	}
	if section.Options == nil {
		section.Options = map[string]string{}
	}
	section.Options[router.ManagedOption] = "1"
	if err := r.client.ApplySections(ctx, router.Configuration{section}, nil, r.secrets(values)); err != nil {
		resp.Diagnostics.AddError("Cannot apply section", err.Error())
		return
	}
	object, diagnostics := r.stateObject(values, section)
	resp.Diagnostics.Append(diagnostics...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, object)...)
	}
}

func (r *typedResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var object types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &object)...)
	if resp.Diagnostics.HasError() || object.IsNull() {
		return
	}
	values := object.Attributes()
	selector := r.spec.Selector
	if r.spec.ReadSelector != nil {
		selector = r.spec.ReadSelector
	}
	section, err := selector(values)
	if err != nil {
		resp.Diagnostics.AddError("Cannot build section selector", err.Error())
		return
	}
	actual, found, err := r.client.ReadSection(ctx, section)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read section before destroy", err.Error())
		return
	}
	if !found {
		return
	}
	deleteBehavior := "forget"
	if value, ok := values["delete_behavior"].(types.String); ok && !value.IsNull() && !value.IsUnknown() {
		deleteBehavior = value.ValueString()
	}
	if deleteBehavior == "delete" && actual.Name != "" && !isAnonymousSection(actual.Name) {
		absent := router.Section{Package: actual.Package, Name: actual.Name, Type: actual.Type, Absent: true}
		if err := r.client.ApplySections(ctx, router.Configuration{absent}, nil, nil); err != nil {
			resp.Diagnostics.AddError("Cannot delete section", err.Error())
		}
		return
	}
	if err := r.client.ForgetSection(ctx, section); err != nil {
		resp.Diagnostics.AddError("Cannot release section ownership", err.Error())
	}
}

func (r *typedResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	values, err := r.spec.Import(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	id, err := r.spec.ID(values)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	values["id"] = types.StringValue(id)
	values["uci_section"] = types.StringNull()
	values["delete_behavior"] = types.StringValue("forget")
	for name, valueType := range r.attrTypes {
		if _, ok := values[name]; !ok {
			values[name] = nullValue(valueType)
		}
	}
	for _, name := range r.spec.WriteOnly {
		if valueType, ok := r.attrTypes[name]; ok {
			values[name] = nullValue(valueType)
		}
	}
	object, diagnostics := types.ObjectValue(r.attrTypes, values)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, object)...)
}

func (r *typedResource) validate(values map[string]attr.Value) error {
	if r.spec.Validate == nil {
		return nil
	}
	return r.spec.Validate(values)
}

func (r *typedResource) stateObject(planned map[string]attr.Value, actual router.Section) (types.Object, diag.Diagnostics) {
	values := map[string]attr.Value{}
	for key, value := range planned {
		values[key] = value
	}
	loaded := r.spec.Load(actual)
	for key, value := range loaded {
		values[key] = value
	}
	for key, valueType := range r.attrTypes {
		if _, ok := values[key]; !ok {
			values[key] = nullValue(valueType)
		}
	}
	for _, key := range r.spec.WriteOnly {
		if valueType, ok := r.attrTypes[key]; ok {
			values[key] = nullValue(valueType)
		}
	}
	id, err := r.spec.ID(planned)
	if err != nil {
		return types.Object{}, diag.Diagnostics{diag.NewErrorDiagnostic("Cannot determine section identity", err.Error())}
	}
	values["id"] = types.StringValue(id)
	values["uci_section"] = types.StringValue(actual.Name)
	deleteBehavior, ok := planned["delete_behavior"].(types.String)
	if !ok || deleteBehavior.IsNull() || deleteBehavior.IsUnknown() {
		deleteBehavior = types.StringValue("forget")
	}
	values["delete_behavior"] = deleteBehavior
	return types.ObjectValue(r.attrTypes, values)
}

func (r *typedResource) secrets(values map[string]attr.Value) map[string]string {
	if r.spec.Secrets == nil {
		return nil
	}
	return r.spec.Secrets(values)
}

func isAnonymousSection(name string) bool {
	return len(name) > 1 && name[0] == '@'
}

type deleteBehaviorValidator struct{}

func (deleteBehaviorValidator) Description(context.Context) string {
	return "value must be forget or delete"
}

func (deleteBehaviorValidator) MarkdownDescription(context.Context) string {
	return "value must be `forget` or `delete`"
}

func (deleteBehaviorValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value != "forget" && value != "delete" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid delete_behavior", "delete_behavior must be forget or delete.")
	}
}
