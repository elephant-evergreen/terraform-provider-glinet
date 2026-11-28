package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"pgregory.net/rapid"
)

var propertyPool = []string{
	"0", "1", "lan", "wan", "guest", "ap", "none", "REJECT", "ACCEPT", "DROP",
	"ipv4", "ipv6", "any", "sae", "psk2", "auto", "6", "11", "HE40", "*",
	"example", "192.168.9.1", "02:00:00:00:00:01", "ethernet", "static",
}

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
				if resource.spec.Validate != nil {
					if err := resource.spec.Validate(values); err != nil {
						return
					}
				}
				if resource.spec.ValidateConfig != nil {
					if err := resource.spec.ValidateConfig(values); err != nil {
						return
					}
				}
				section, err := resource.spec.Selector(values)
				if err != nil {
					t.Fatalf("%s cannot select an accepted configuration: %v", resource.spec.TypeName, err)
				}
				loaded := resource.spec.Load(section)
				second, err := resource.spec.Selector(loaded)
				if err != nil {
					t.Fatalf("%s cannot select the values it loaded: %v", resource.spec.TypeName, err)
				}
				if !preservesMeaning(section, second) {
					t.Fatalf("%s lost or changed a managed setting in a selector/load round trip:\nfirst:  %#v\nsecond: %#v", resource.spec.TypeName, section, second)
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

// preservesMeaning keeps every explicit setting through a selector/load round
// trip. Empty values may resolve to a firmware default on read, and lists are
// compared as multisets because set attributes do not preserve order.
func preservesMeaning(first, second router.Section) bool {
	if first.Package != second.Package || first.Name != second.Name || first.Type != second.Type || first.Absent != second.Absent || first.Missing != second.Missing {
		return false
	}
	for key, value := range first.Options {
		if value == "" {
			continue
		}
		if second.Options[key] != value {
			return false
		}
	}
	for key, values := range first.Lists {
		if len(values) == 0 {
			continue
		}
		other, ok := second.Lists[key]
		if !ok || !sameStrings(values, other) {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string{}, a...)
	right := append([]string{}, b...)
	sort.Strings(left)
	sort.Strings(right)
	return reflect.DeepEqual(left, right)
}

func generateAttributeValue(t *rapid.T, name string, attribute schema.Attribute) attr.Value {
	if rapid.Bool().Draw(t, name+"-default") {
		if value, ok := schemaDefaultValue(attribute); ok {
			return value
		}
	}
	switch attribute := attribute.(type) {
	case schema.StringAttribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.StringNull()
		}
		for attempt := 0; attempt < 8; attempt++ {
			var candidate types.String
			if rapid.Bool().Draw(t, fmt.Sprintf("%s-%d-pool", name, attempt)) {
				candidate = types.StringValue(rapid.SampledFrom(propertyPool).Draw(t, fmt.Sprintf("%s-%d", name, attempt)))
			} else {
				candidate = types.StringValue(rapid.StringMatching(`^[A-Za-z0-9 ._:/'"$-]{0,16}$`).Draw(t, fmt.Sprintf("%s-%d", name, attempt)))
			}
			if attribute.Required && candidate.ValueString() == "" {
				continue
			}
			if stringValid(attribute.Validators, candidate) {
				return candidate
			}
		}
		return defaultOrNullString(attribute)
	case schema.Int64Attribute:
		if attribute.Optional && rapid.Bool().Draw(t, name+"-null") {
			return types.Int64Null()
		}
		for attempt := 0; attempt < 8; attempt++ {
			candidate := types.Int64Value(rapid.Int64Range(-5, 1000).Draw(t, fmt.Sprintf("%s-%d", name, attempt)))
			if int64Valid(attribute.Validators, candidate) {
				return candidate
			}
		}
		return defaultOrNullInt64(attribute)
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
			value := rapid.SampledFrom(propertyPool).Draw(t, fmt.Sprintf("%s-%d", name, index))
			elements = append(elements, types.StringValue(value))
		}
		candidate := types.SetValueMust(attribute.ElementType, elements)
		for _, check := range attribute.Validators {
			var response validator.SetResponse
			check.ValidateSet(context.Background(), validator.SetRequest{ConfigValue: candidate, Path: path.Root(name)}, &response)
			if response.Diagnostics.HasError() {
				return defaultOrNullSet(attribute)
			}
		}
		return candidate
	default:
		return types.StringNull()
	}
}

func stringValid(validators []validator.String, value types.String) bool {
	for _, check := range validators {
		var response validator.StringResponse
		check.ValidateString(context.Background(), validator.StringRequest{ConfigValue: value, Path: path.Root("value")}, &response)
		if response.Diagnostics.HasError() {
			return false
		}
	}
	return true
}

func int64Valid(validators []validator.Int64, value types.Int64) bool {
	for _, check := range validators {
		var response validator.Int64Response
		check.ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: value, Path: path.Root("value")}, &response)
		if response.Diagnostics.HasError() {
			return false
		}
	}
	return true
}

func defaultOrNullString(attribute schema.StringAttribute) attr.Value {
	value, ok := schemaDefaultValue(attribute)
	if ok && value != nil {
		return value
	}
	for _, check := range attribute.Validators {
		if allowed, ok := check.(oneOfStringValidator); ok && len(allowed.allowed) > 0 {
			return types.StringValue(allowed.allowed[0])
		}
	}
	return types.StringNull()
}

func defaultOrNullInt64(attribute schema.Int64Attribute) attr.Value {
	value, ok := schemaDefaultValue(attribute)
	if !ok || value == nil {
		return types.Int64Null()
	}
	return value
}

func defaultOrNullSet(attribute schema.SetAttribute) attr.Value {
	value, ok := schemaDefaultValue(attribute)
	if !ok || value == nil {
		return types.SetNull(attribute.ElementType)
	}
	return value
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
