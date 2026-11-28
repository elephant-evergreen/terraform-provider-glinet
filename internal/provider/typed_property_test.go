package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"pgregory.net/rapid"
)

func TestTypedSelectorLoadIsStable(t *testing.T) {
	ctx := context.Background()
	for _, factory := range New("test")().Resources(ctx) {
		resource, ok := factory().(*typedResource)
		if !ok {
			continue
		}
		var response frameworkresource.SchemaResponse
		resource.Schema(ctx, frameworkresource.SchemaRequest{}, &response)
		attributes := response.Schema.Attributes
		t.Run(resource.spec.TypeName, func(t *testing.T) {
			rapid.Check(t, func(t *rapid.T) {
				values := make(map[string]attr.Value, len(attributes))
				for name, attribute := range attributes {
					values[name] = generateAttributeValue(t, name, attribute)
				}
				section, err := resource.spec.Selector(values)
				if err != nil {
					return
				}
				if !router.OptionAllowed(section.Package, section.Type, "") && section.Package != "" {
					normalized := router.Section{Package: section.Package, Type: section.Type}
					_ = normalized
				}
				for key := range section.Options {
					if key == router.ManagedOption {
						continue
					}
					if !router.OptionAllowed(section.Package, section.Type, key) {
						t.Fatalf("%s writes unregistered option %s.%s.%s", resource.spec.TypeName, section.Package, section.Type, key)
					}
				}
				for key := range section.Lists {
					if !router.OptionAllowed(section.Package, section.Type, key) {
						t.Fatalf("%s writes unregistered list %s.%s.%s", resource.spec.TypeName, section.Package, section.Type, key)
					}
				}
				loaded := resource.spec.Load(section)
				second, err := resource.spec.Selector(loaded)
				if err != nil {
					return
				}
				third, err := resource.spec.Selector(resource.spec.Load(second))
				if err != nil {
					return
				}
				if !reflect.DeepEqual(second, third) {
					t.Fatalf("%s selector does not reach a stable form:\nsecond: %#v\nthird: %#v", resource.spec.TypeName, second, third)
				}
			})
		})
	}
}

func generateAttributeValue(t *rapid.T, name string, attribute schema.Attribute) attr.Value {
	if value, ok := schemaDefaultValue(attribute); ok {
		return value
	}
	switch attribute := attribute.(type) {
	case schema.StringAttribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.StringNull()
		}
		return types.StringValue(rapid.StringMatching(`^[A-Za-z0-9 ._:/'"$-]{0,16}$`).Draw(t, name))
	case schema.Int64Attribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.Int64Null()
		}
		return types.Int64Value(rapid.Int64Range(-5, 1000).Draw(t, name))
	case schema.BoolAttribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.BoolNull()
		}
		return types.BoolValue(rapid.Bool().Draw(t, name))
	case schema.SetAttribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.SetNull(attribute.ElementType)
		}
		count := rapid.IntRange(0, 3).Draw(t, name+"-count")
		elements := make([]attr.Value, 0, count)
		for index := 0; index < count; index++ {
			elements = append(elements, types.StringValue(rapid.StringMatching(`^[A-Za-z0-9._:/-]{0,8}$`).Draw(t, fmt.Sprintf("%s-%d", name, index))))
		}
		return types.SetValueMust(attribute.ElementType, elements)
	default:
		return types.StringNull()
	}
}

func schemaDefaultValue(attribute schema.Attribute) (attr.Value, bool) {
	ctx := context.Background()
	switch attribute := attribute.(type) {
	case schema.StringAttribute:
		if attribute.Default == nil {
			return nil, false
		}
		var response defaults.StringResponse
		attribute.Default.DefaultString(ctx, defaults.StringRequest{}, &response)
		return response.PlanValue, !response.Diagnostics.HasError()
	case schema.Int64Attribute:
		if attribute.Default == nil {
			return nil, false
		}
		var response defaults.Int64Response
		attribute.Default.DefaultInt64(ctx, defaults.Int64Request{}, &response)
		return response.PlanValue, !response.Diagnostics.HasError()
	case schema.BoolAttribute:
		if attribute.Default == nil {
			return nil, false
		}
		var response defaults.BoolResponse
		attribute.Default.DefaultBool(ctx, defaults.BoolRequest{}, &response)
		return response.PlanValue, !response.Diagnostics.HasError()
	case schema.SetAttribute:
		if attribute.Default == nil {
			return nil, false
		}
		var response defaults.SetResponse
		attribute.Default.DefaultSet(ctx, defaults.SetRequest{}, &response)
		return response.PlanValue, !response.Diagnostics.HasError()
	}
	return nil, false
}
