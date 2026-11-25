package provider

import (
	"context"
	"fmt"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type firewallRulesModel struct {
	ID    types.String `tfsdk:"id"`
	Rules types.List   `tfsdk:"rules"`
}

type firewallRuleInput struct {
	Name      string
	Existing  bool
	Src       string
	Dest      string
	SrcIPs    []string
	DestIPs   []string
	Protocols []string
	SrcPorts  []string
	DestPorts []string
	Family    string
	Target    string
	Enabled   bool
}

func NewFirewallRulesResource() resource.Resource {
	return &firewallRulesResource{}
}

type firewallRulesResource struct {
	client *router.Client
}

var (
	_ resource.Resource                = &firewallRulesResource{}
	_ resource.ResourceWithConfigure   = &firewallRulesResource{}
	_ resource.ResourceWithImportState = &firewallRulesResource{}
)

func (r *firewallRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rules"
}

func (r *firewallRulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func firewallRuleAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":       types.StringType,
		"existing":   types.BoolType,
		"src":        types.StringType,
		"dest":       types.StringType,
		"src_ips":    types.SetType{ElemType: types.StringType},
		"dest_ips":   types.SetType{ElemType: types.StringType},
		"protocols":  types.SetType{ElemType: types.StringType},
		"src_ports":  types.SetType{ElemType: types.StringType},
		"dest_ports": types.SetType{ElemType: types.StringType},
		"family":     types.StringType,
		"target":     types.StringType,
		"enabled":    types.BoolType,
	}
}

func firewallRuleObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: firewallRuleAttributeTypes()}
}

func firewallRulesSchema() schema.Schema {
	attributes := map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Unique firewall rule name. The name is the stable identity in the ordered policy.",
		},
		"existing": schema.BoolAttribute{
			Optional: true, Computed: true, Default: booldefault.StaticBool(false),
			MarkdownDescription: "When true, preserve the current rule content and manage only its order. Use this for stock rules that are not fully managed by Terraform.",
		},
		"src": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString(""),
			MarkdownDescription: "Source zone, `*`, or empty.",
		},
		"dest": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString(""),
			MarkdownDescription: "Destination zone, `*`, or empty.",
		},
		"src_ips": schema.SetAttribute{
			Optional: true, Computed: true, ElementType: types.StringType,
			Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			MarkdownDescription: "Source addresses or CIDRs.",
		},
		"dest_ips": schema.SetAttribute{
			Optional: true, Computed: true, ElementType: types.StringType,
			Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			MarkdownDescription: "Destination addresses or CIDRs.",
		},
		"protocols": schema.SetAttribute{
			Optional: true, Computed: true, ElementType: types.StringType,
			Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			MarkdownDescription: "Protocols, such as `tcp`, `udp`, `icmp`, or `all`.",
		},
		"src_ports": schema.SetAttribute{
			Optional: true, Computed: true, ElementType: types.StringType,
			Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			MarkdownDescription: "Source ports or ranges.",
		},
		"dest_ports": schema.SetAttribute{
			Optional: true, Computed: true, ElementType: types.StringType,
			Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			MarkdownDescription: "Destination ports or ranges.",
		},
		"family": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString("ipv4"),
			MarkdownDescription: "Address family: `ipv4`, `ipv6`, or `any`.",
		},
		"target": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString("REJECT"),
			MarkdownDescription: "Target: ACCEPT, REJECT, DROP, or LOG.",
		},
		"enabled": schema.BoolAttribute{
			Optional: true, Computed: true, Default: booldefault.StaticBool(true),
			MarkdownDescription: "Whether the rule is active.",
		},
	}
	return schema.Schema{
		MarkdownDescription: "Manage the complete ordered firewall rule set. Rule content, membership, and order are written in one recovery-protected transaction. Stock rules are listed with `existing = true` and keep their current content.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Stable identifier."},
			"rules": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Firewall rules in UCI order. Every existing rule must appear exactly once.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: attributes,
				},
			},
		},
	}
}

func (r *firewallRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = firewallRulesSchema()
}

func (r *firewallRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || plan.Rules.IsNull() || plan.Rules.IsUnknown() {
		return
	}
	if err := r.apply(ctx, plan.Rules, types.ListNull(firewallRuleObjectType())); err != nil {
		resp.Diagnostics.AddError("Cannot apply firewall rules", err.Error())
		return
	}
	resp.Diagnostics.Append(r.read(ctx, &plan)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *firewallRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || state.Rules.IsNull() {
		return
	}
	resp.Diagnostics.Append(r.read(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}

func (r *firewallRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state firewallRulesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || plan.Rules.IsNull() || plan.Rules.IsUnknown() || state.Rules.IsNull() {
		return
	}
	if err := r.apply(ctx, plan.Rules, state.Rules); err != nil {
		resp.Diagnostics.AddError("Cannot apply firewall rules", err.Error())
		return
	}
	resp.Diagnostics.Append(r.read(ctx, &plan)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *firewallRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRulesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || state.Rules.IsNull() {
		return
	}
	rules, err := r.client.FirewallRules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read firewall rules", err.Error())
		return
	}
	changes := router.Configuration{}
	for _, rule := range rules {
		if rule.Options == nil {
			rule.Options = map[string]string{}
		}
		rule.Options[router.ManagedOption] = "0"
		changes = append(changes, rule)
	}
	if err := r.client.ApplySections(ctx, changes, nil); err != nil {
		resp.Diagnostics.AddError("Cannot release firewall rule ownership", err.Error())
	}
}

func (r *firewallRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "firewall_rules" {
		resp.Diagnostics.AddError("Invalid import ID", "Use firewall_rules.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &firewallRulesModel{
		ID:    types.StringValue("firewall_rules"),
		Rules: types.ListValueMust(firewallRuleObjectType(), nil),
	})...)
}

func (r *firewallRulesResource) apply(ctx context.Context, planRules, stateRules types.List) error {
	planInputs, diagnostics := parseFirewallRuleInputs(ctx, planRules)
	if diagnostics.HasError() {
		return fmt.Errorf("invalid planned firewall rules")
	}
	actualRules, err := r.client.FirewallRules(ctx)
	if err != nil {
		return err
	}
	actualByName := map[string]router.Section{}
	for _, actual := range actualRules {
		actualByName[actual.Name] = actual
	}
	desired := router.Configuration{}
	planNames := map[string]bool{}
	for _, input := range planInputs {
		if input.Name == "" {
			return fmt.Errorf("firewall rule name is required")
		}
		if planNames[input.Name] {
			return fmt.Errorf("firewall rule %s is listed more than once", input.Name)
		}
		planNames[input.Name] = true
		if input.Existing {
			actual, ok := actualByName[input.Name]
			if !ok {
				return fmt.Errorf("existing firewall rule %s is not present; import it or remove the entry", input.Name)
			}
			if actual.Options == nil {
				actual.Options = map[string]string{}
			}
			actual.Options[router.ManagedOption] = "1"
			desired = append(desired, actual)
			continue
		}
		section, err := firewallRuleSection(input)
		if err != nil {
			return err
		}
		desired = append(desired, section)
	}
	removed := router.Configuration{}
	if !stateRules.IsNull() && !stateRules.IsUnknown() {
		stateInputs, diagnostics := parseFirewallRuleInputs(ctx, stateRules)
		if diagnostics.HasError() {
			return fmt.Errorf("invalid prior firewall rules")
		}
		for _, input := range stateInputs {
			if planNames[input.Name] {
				continue
			}
			if input.Existing {
				return fmt.Errorf("existing firewall rule %s cannot be removed from the ordered policy; keep it listed", input.Name)
			}
			removed = append(removed, router.Section{Package: "firewall", Name: input.Name, Type: "rule", Absent: true})
		}
	}
	return r.client.ApplyFirewallRules(ctx, desired, removed)
}

func (r *firewallRulesResource) read(ctx context.Context, model *firewallRulesModel) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	actualRules, err := r.client.FirewallRules(ctx)
	if err != nil {
		diagnostics.AddError("Cannot read firewall rules", err.Error())
		return diagnostics
	}
	existing := map[string]bool{}
	if !model.Rules.IsNull() && !model.Rules.IsUnknown() {
		inputs, d := parseFirewallRuleInputs(ctx, model.Rules)
		diagnostics.Append(d...)
		if diagnostics.HasError() {
			return diagnostics
		}
		for _, input := range inputs {
			existing[input.Name] = input.Existing
		}
	}
	elements := make([]attr.Value, 0, len(actualRules))
	for _, actual := range actualRules {
		input := firewallRuleInputFromSection(actual)
		if value, ok := existing[actual.Name]; ok {
			input.Existing = value
		} else {
			input.Existing = true
		}
		object, d := firewallRuleObject(input)
		diagnostics.Append(d...)
		if diagnostics.HasError() {
			return diagnostics
		}
		elements = append(elements, object)
	}
	model.ID = types.StringValue("firewall_rules")
	model.Rules = types.ListValueMust(firewallRuleObjectType(), elements)
	return diagnostics
}

func parseFirewallRuleInputs(ctx context.Context, list types.List) ([]firewallRuleInput, diag.Diagnostics) {
	var objects []types.Object
	diagnostics := list.ElementsAs(ctx, &objects, false)
	if diagnostics.HasError() {
		return nil, diagnostics
	}
	inputs := make([]firewallRuleInput, 0, len(objects))
	for _, object := range objects {
		values := object.Attributes()
		inputs = append(inputs, firewallRuleInput{
			Name:      stringOf(values, "name"),
			Existing:  boolOf(values, "existing", false),
			Src:       stringOf(values, "src"),
			Dest:      stringOf(values, "dest"),
			SrcIPs:    stringsOf(values, "src_ips"),
			DestIPs:   stringsOf(values, "dest_ips"),
			Protocols: stringsOf(values, "protocols"),
			SrcPorts:  stringsOf(values, "src_ports"),
			DestPorts: stringsOf(values, "dest_ports"),
			Family:    stringOf(values, "family"),
			Target:    stringOf(values, "target"),
			Enabled:   boolOf(values, "enabled", true),
		})
	}
	return inputs, diagnostics
}

func firewallRuleSection(input firewallRuleInput) (router.Section, error) {
	switch input.Target {
	case "ACCEPT", "REJECT", "DROP", "LOG":
	default:
		return router.Section{}, fmt.Errorf("rule %s target must be ACCEPT, REJECT, DROP, or LOG", input.Name)
	}
	switch input.Family {
	case "ipv4", "ipv6", "any":
	default:
		return router.Section{}, fmt.Errorf("rule %s family must be ipv4, ipv6, or any", input.Name)
	}
	options := map[string]string{
		"name": input.Name, "family": input.Family, "target": input.Target,
		"enabled": boolString(input.Enabled), "src": input.Src, "dest": input.Dest,
		"proto": joinStrings(input.Protocols), "src_port": joinStrings(input.SrcPorts), "dest_port": joinStrings(input.DestPorts),
		router.ManagedOption: "1",
	}
	section := router.Section{Package: "firewall", Type: "rule", Name: input.Name, Options: options, Lists: map[string][]string{}}
	for _, field := range []struct {
		values []string
		option string
	}{{input.SrcIPs, "src_ip"}, {input.DestIPs, "dest_ip"}} {
		switch len(field.values) {
		case 0:
			options[field.option] = ""
		case 1:
			options[field.option] = field.values[0]
		default:
			section.Lists[field.option] = field.values
		}
	}
	return section, nil
}

func firewallRuleInputFromSection(section router.Section) firewallRuleInput {
	input := firewallRuleInput{
		Name:      section.Options["name"],
		Src:       section.Options["src"],
		Dest:      section.Options["dest"],
		SrcIPs:    append(splitFields(section.Options["src_ip"]), section.Lists["src_ip"]...),
		DestIPs:   append(splitFields(section.Options["dest_ip"]), section.Lists["dest_ip"]...),
		Protocols: splitFields(section.Options["proto"]),
		SrcPorts:  splitFields(section.Options["src_port"]),
		DestPorts: splitFields(section.Options["dest_port"]),
		Family:    defaultString(section.Options["family"], "ipv4"),
		Target:    defaultString(section.Options["target"], "REJECT"),
		Enabled:   section.Options["enabled"] != "0",
	}
	if input.Name == "" {
		input.Name = section.Name
	}
	return input
}

func firewallRuleObject(input firewallRuleInput) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(firewallRuleAttributeTypes(), map[string]attr.Value{
		"name":       stringValue(input.Name),
		"existing":   boolValue(input.Existing),
		"src":        stringValue(input.Src),
		"dest":       stringValue(input.Dest),
		"src_ips":    stringSet(input.SrcIPs),
		"dest_ips":   stringSet(input.DestIPs),
		"protocols":  stringSet(input.Protocols),
		"src_ports":  stringSet(input.SrcPorts),
		"dest_ports": stringSet(input.DestPorts),
		"family":     stringValue(input.Family),
		"target":     stringValue(input.Target),
		"enabled":    boolValue(input.Enabled),
	})
}
