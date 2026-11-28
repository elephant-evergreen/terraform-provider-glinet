package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestTypedStateUpgraderWiring(t *testing.T) {
	ctx := context.Background()
	called := false
	spec := typedSpec{
		TypeName:      "upgrade_probe",
		SchemaVersion: 1,
		Attributes:    map[string]schema.Attribute{"value": schema.StringAttribute{Optional: true}},
		Selector:      func(map[string]attr.Value) (router.Section, error) { return router.Section{}, nil },
		Load:          func(router.Section) map[string]attr.Value { return nil },
		ID:            func(map[string]attr.Value) (string, error) { return "probe", nil },
		UpgradeState: func(_ context.Context, _ frameworkresource.UpgradeStateRequest, resp *frameworkresource.UpgradeStateResponse) {
			called = true
			resp.Diagnostics.AddWarning("upgraded", "probe")
		},
	}
	resource := newTypedResource(spec).(*typedResource)
	var schemaResponse frameworkresource.SchemaResponse
	resource.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResponse)
	if schemaResponse.Schema.Version != 1 {
		t.Fatalf("schema version = %d, want 1", schemaResponse.Schema.Version)
	}
	upgraders := resource.UpgradeState(ctx)
	upgrader, ok := upgraders[0]
	if !ok {
		t.Fatalf("missing upgrader for version 0: %#v", upgraders)
	}
	var upgradeResponse frameworkresource.UpgradeStateResponse
	upgrader.StateUpgrader(ctx, frameworkresource.UpgradeStateRequest{}, &upgradeResponse)
	if !called {
		t.Fatal("state upgrader was not called")
	}
	if len(upgradeResponse.Diagnostics) != 1 {
		t.Fatalf("upgrader diagnostics = %#v", upgradeResponse.Diagnostics)
	}
}

func TestTypedStateUpgraderAbsentByDefault(t *testing.T) {
	ctx := context.Background()
	base := typedSpec{
		TypeName:   "upgrade_probe",
		Attributes: map[string]schema.Attribute{},
		Selector:   func(map[string]attr.Value) (router.Section, error) { return router.Section{}, nil },
		Load:       func(router.Section) map[string]attr.Value { return nil },
		ID:         func(map[string]attr.Value) (string, error) { return "probe", nil },
	}
	versioned := base
	versioned.SchemaVersion = 1
	for name, spec := range map[string]typedSpec{"no version": base, "version without upgrader": versioned} {
		t.Run(name, func(t *testing.T) {
			resource := newTypedResource(spec).(*typedResource)
			if upgraders := resource.UpgradeState(ctx); upgraders != nil {
				t.Fatalf("unexpected upgraders: %#v", upgraders)
			}
		})
	}
}
