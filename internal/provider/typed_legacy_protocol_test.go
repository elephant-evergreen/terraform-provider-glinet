package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// legacySystemNTPState is the shape a v0.3.0 or v0.3.1 state has: the section
// carries the managed marker and no owner token, and the computed owner
// attribute is null.
func legacySystemNTPState() router.Configuration {
	return router.Configuration{{
		Package: "system",
		Name:    "ntp",
		Type:    "timeserver",
		Options: map[string]string{"enabled": "1", "use_dhcp": "0", "interface": "", router.ManagedOption: "1"},
		Lists:   map[string][]string{"server": {"time.example"}},
	}}
}

func diagnosticsFailed(diagnostics []*tfprotov6.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func protocolObject(t *testing.T, objectType tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	object, ok := objectType.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type %s is not an object", objectType)
	}
	attributes := map[string]tftypes.Value{}
	for name, attributeType := range object.AttributeTypes {
		if value, ok := set[name]; ok {
			attributes[name] = value
		} else {
			attributes[name] = tftypes.NewValue(attributeType, nil)
		}
	}
	return tftypes.NewValue(objectType, attributes)
}

func attributeValue(t *testing.T, value tftypes.Value, name string) tftypes.Value {
	t.Helper()
	attributes := map[string]tftypes.Value{}
	if err := value.As(&attributes); err != nil {
		t.Fatalf("cannot read %s from a protocol value: %v", name, err)
	}
	return attributes[name]
}

func protocolServer(t *testing.T, remote *typedFixtureRunner) tfprotov6.ProviderServer {
	t.Helper()
	client := router.NewClient(remote, fixtureProtection{})
	client.AllowNetworkDisruption(true)
	factory := providerserver.NewProtocol6WithError(&fixtureProvider{Provider: New("test")(), client: client})
	server, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// TestLegacyStateFirstUpdateKeepsPlanConsistent drives PlanResourceChange and
// ApplyResourceChange over the provider protocol. Terraform Core rejects an
// apply whose result differs from a known planned value, so the test compares
// every known planned attribute with the applied state and fails on a
// mismatch, the same way "Provider produced inconsistent result after apply"
// is decided.
func TestLegacyStateFirstUpdateKeepsPlanConsistent(t *testing.T) {
	ctx := context.Background()
	remote := &typedFixtureRunner{actual: legacySystemNTPState()}
	server := protocolServer(t, remote)
	schemaResponse, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	resources := map[string]*tfprotov6.Schema{}
	for name, schema := range schemaResponse.ResourceSchemas {
		resources[name] = schema
	}
	const typeName = "glinet_system_ntp"
	resourceSchema, ok := resources[typeName]
	if !ok {
		t.Fatalf("%s is not registered", typeName)
	}
	resourceType := resourceSchema.ValueType()
	set := tftypes.Set{ElementType: tftypes.String}
	stateValues := map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, "system_ntp"),
		"uci_section":     tftypes.NewValue(tftypes.String, "ntp"),
		"delete_behavior": tftypes.NewValue(tftypes.String, "forget"),
		"owner":           tftypes.NewValue(tftypes.String, nil),
		"enabled":         tftypes.NewValue(tftypes.Bool, true),
		"servers":         tftypes.NewValue(set, []tftypes.Value{tftypes.NewValue(tftypes.String, "time.example")}),
		"use_dhcp":        tftypes.NewValue(tftypes.Bool, false),
		"interface":       tftypes.NewValue(tftypes.String, ""),
	}
	prior := protocolObject(t, resourceType, stateValues)
	configValues := map[string]tftypes.Value{
		"servers": tftypes.NewValue(set, []tftypes.Value{tftypes.NewValue(tftypes.String, "time2.example")}),
	}
	config := protocolObject(t, resourceType, configValues)
	proposedValues := map[string]tftypes.Value{}
	for name := range resourceType.(tftypes.Object).AttributeTypes {
		proposedValues[name] = stateValues[name]
	}
	proposedValues["servers"] = configValues["servers"]
	proposed := protocolObject(t, resourceType, proposedValues)
	priorDynamic, _ := tfprotov6.NewDynamicValue(resourceType, prior)
	proposedDynamic, _ := tfprotov6.NewDynamicValue(resourceType, proposed)
	configDynamic, _ := tfprotov6.NewDynamicValue(resourceType, config)
	providerState, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{TerraformVersion: "1.17.2"})
	if err != nil {
		t.Fatal(err)
	}
	if providerState != nil && diagnosticsFailed(providerState.Diagnostics) {
		t.Fatalf("provider configuration failed: %v", providerState.Diagnostics)
	}
	plan, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorDynamic,
		ProposedNewState: &proposedDynamic,
		Config:           &configDynamic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnosticsFailed(plan.Diagnostics) {
		t.Fatalf("plan failed: %v", plan.Diagnostics)
	}
	planned, err := plan.PlannedState.Unmarshal(resourceType)
	if err != nil {
		t.Fatal(err)
	}
	if owner := attributeValue(t, planned, "owner"); owner.IsKnown() {
		t.Fatalf("owner is known in the plan before the token is adopted: %s", owner)
	}
	applied, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName:       typeName,
		PriorState:     &priorDynamic,
		PlannedState:   plan.PlannedState,
		Config:         &configDynamic,
		PlannedPrivate: plan.PlannedPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnosticsFailed(applied.Diagnostics) {
		t.Fatalf("apply failed: %v", applied.Diagnostics)
	}
	newState, err := applied.NewState.Unmarshal(resourceType)
	if err != nil {
		t.Fatal(err)
	}
	plannedAttributes := map[string]tftypes.Value{}
	appliedAttributes := map[string]tftypes.Value{}
	if err := planned.As(&plannedAttributes); err != nil {
		t.Fatal(err)
	}
	if err := newState.As(&appliedAttributes); err != nil {
		t.Fatal(err)
	}
	for name, plannedValue := range plannedAttributes {
		if !plannedValue.IsKnown() {
			continue
		}
		if !plannedValue.Equal(appliedAttributes[name]) {
			t.Fatalf("provider produced an inconsistent result for %s: planned %s, applied %s", name, plannedValue, appliedAttributes[name])
		}
	}
	owner := attributeValue(t, newState, "owner")
	if !owner.IsKnown() {
		t.Fatal("the first update did not record an owner token")
	}
	var token string
	if err := owner.As(&token); err != nil || token == "" {
		t.Fatalf("owner token = %q (%v)", token, err)
	}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	if len(remote.actual) != 1 || remote.actual[0].Options[router.OwnerTokenOption] != token {
		t.Fatalf("router token does not match the applied state: %+v", remote.actual)
	}
}

func TestLegacyStateWithoutConfigChangePlansNoUpdate(t *testing.T) {
	ctx := context.Background()
	remote := &typedFixtureRunner{actual: legacySystemNTPState()}
	server := protocolServer(t, remote)
	schemaResponse, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	const typeName = "glinet_system_ntp"
	resourceType := schemaResponse.ResourceSchemas[typeName].ValueType()
	set := tftypes.Set{ElementType: tftypes.String}
	stateValues := map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, "system_ntp"),
		"uci_section":     tftypes.NewValue(tftypes.String, "ntp"),
		"delete_behavior": tftypes.NewValue(tftypes.String, "forget"),
		"owner":           tftypes.NewValue(tftypes.String, nil),
		"enabled":         tftypes.NewValue(tftypes.Bool, true),
		"servers":         tftypes.NewValue(set, []tftypes.Value{tftypes.NewValue(tftypes.String, "time.example")}),
		"use_dhcp":        tftypes.NewValue(tftypes.Bool, false),
		"interface":       tftypes.NewValue(tftypes.String, ""),
	}
	prior := protocolObject(t, resourceType, stateValues)
	config := protocolObject(t, resourceType, map[string]tftypes.Value{"servers": stateValues["servers"]})
	proposedValues := map[string]tftypes.Value{}
	for name := range resourceType.(tftypes.Object).AttributeTypes {
		proposedValues[name] = stateValues[name]
	}
	proposed := protocolObject(t, resourceType, proposedValues)
	priorDynamic, _ := tfprotov6.NewDynamicValue(resourceType, prior)
	proposedDynamic, _ := tfprotov6.NewDynamicValue(resourceType, proposed)
	configDynamic, _ := tfprotov6.NewDynamicValue(resourceType, config)
	if _, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{TerraformVersion: "1.17.2"}); err != nil {
		t.Fatal(err)
	}
	plan, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         typeName,
		PriorState:       &priorDynamic,
		ProposedNewState: &proposedDynamic,
		Config:           &configDynamic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diagnosticsFailed(plan.Diagnostics) {
		t.Fatalf("plan failed: %v", plan.Diagnostics)
	}
	planned, err := plan.PlannedState.Unmarshal(resourceType)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.Equal(prior) {
		t.Fatalf("a legacy state without a configuration change planned an update; Terraform would apply the owner token against a null plan:\nplanned: %s\nprior: %s", planned, prior)
	}
}
