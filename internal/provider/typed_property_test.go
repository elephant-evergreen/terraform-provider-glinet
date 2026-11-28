package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
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

// propertyHints biases well-known attribute names toward values the
// resource's own validation accepts, so format-checked resources are actually
// exercised instead of being rejected in every check.
var propertyHints = map[string][]string{
	"mac":       {"02:00:00:00:00:01", "02:00:00:00:00:02"},
	"ip":        {"192.0.2.10", "192.0.2.11"},
	"target":    {"192.0.2.0/24", "2001:db8::/32", "10.0.0.0/8"},
	"gateway":   {"192.0.2.1", "2001:db8::1"},
	"hostname":  {"example-host", "router"},
	"section":   {"example", "guest"},
	"name":      {"example", "guest"},
	"interface": {"lan", "wan"},
}

var propertyPool = []string{
	"0", "1", "lan", "wan", "guest", "ap", "none", "REJECT", "ACCEPT", "DROP",
	"ipv4", "ipv6", "any", "sae", "psk2", "auto", "6", "11", "HE40", "*",
	"example", "example-host", "example.com", "192.168.9.1", "02:00:00:00:00:01",
	"ethernet", "static", "192.0.2.0/24", "2001:db8::/32", "10.0.0.0/8",
	"192.0.2.1", "2001:db8::1", "br-lan", "wg0", "tcpudp", "udp", "icmp",
	"access", "trunk", "server", "client", "pppoe", "dhcp", "max", "medium",
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
			generated, accepted := 0, 0
			rapid.Check(t, func(t *rapid.T) {
				generated++
				values, ok := generateAcceptedValues(t, resource, attributes)
				if !ok {
					return
				}
				accepted++
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
			t.Logf("%s: %d of %d generated configurations passed validation", resource.spec.TypeName, accepted, generated)
			if accepted == 0 {
				t.Fatalf("%s: no generated configuration passed validation, so the property verified nothing", resource.spec.TypeName)
			}
		})
	}
}

// generateAcceptedValues retries value generation until the resource accepts
// the configuration, so the property measures how many checks actually
// exercised the resource instead of silently skipping invalid draws.
func generateAcceptedValues(t *rapid.T, resource *typedResource, attributes map[string]schema.Attribute) (map[string]attr.Value, bool) {
	for attempt := 0; attempt < 8; attempt++ {
		values := make(map[string]attr.Value, len(attributes))
		for name, attribute := range attributes {
			values[name] = generateAttributeValue(t, name, attribute)
		}
		if resource.spec.Validate != nil {
			if err := resource.spec.Validate(values); err != nil {
				continue
			}
		}
		if resource.spec.ValidateConfig != nil {
			if err := resource.spec.ValidateConfig(values); err != nil {
				continue
			}
		}
		return values, true
	}
	return nil, false
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
			if hints, ok := propertyHints[name]; ok && rapid.Bool().Draw(t, fmt.Sprintf("%s-%d-hint", name, attempt)) {
				candidate = types.StringValue(rapid.SampledFrom(hints).Draw(t, fmt.Sprintf("%s-%d", name, attempt)))
			} else if rapid.Bool().Draw(t, fmt.Sprintf("%s-%d-pool", name, attempt)) {
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

// TestWriteOnlyValuesNeverEnterState keeps write-only secrets out of the
// object the provider stores, and out of every other attribute.
func TestWriteOnlyValuesNeverEnterState(t *testing.T) {
	ctx := context.Background()
	for _, factory := range New("test")().Resources(ctx) {
		resource, ok := factory().(*typedResource)
		if !ok || len(resource.spec.WriteOnly) == 0 {
			continue
		}
		var response frameworkresource.SchemaResponse
		resource.Schema(ctx, frameworkresource.SchemaRequest{}, &response)
		attributes := response.Schema.Attributes
		t.Run(resource.spec.TypeName, func(t *testing.T) {
			generated, accepted := 0, 0
			rapid.Check(t, func(t *rapid.T) {
				generated++
				values, ok := generateAcceptedValues(t, resource, attributes)
				if !ok {
					return
				}
				accepted++
				section, err := resource.spec.Selector(values)
				if err != nil {
					return
				}
				secrets := map[string]string{}
				for _, name := range resource.spec.WriteOnly {
					secret := rapid.StringMatching(`^[A-Za-z0-9!#$%&()*+,-./:;<=>?@[\]^_{|}~]{8,64}$`).Draw(t, "secret-"+name)
					secrets[name] = secret
					values[name] = types.StringValue(secret)
				}
				object, diagnostics := resource.stateObject(values, section, "token")
				if diagnostics.HasError() {
					t.Fatalf("cannot build the state: %v", diagnostics)
				}
				applied := object.Attributes()
				for name, secret := range secrets {
					if !applied[name].IsNull() {
						t.Fatalf("write-only %s reached the state", name)
					}
					for attribute, value := range applied {
						if strings.Contains(attributeValueString(value), secret) {
							t.Fatalf("write-only %s leaked into the %s attribute", name, attribute)
						}
					}
				}
			})
			t.Logf("%s: %d of %d generated configurations passed validation", resource.spec.TypeName, accepted, generated)
			if accepted == 0 {
				t.Fatalf("%s: no generated configuration passed validation, so the property verified nothing", resource.spec.TypeName)
			}
		})
	}
}

func attributeValueString(value attr.Value) string {
	switch value := value.(type) {
	case types.String:
		return value.ValueString()
	case types.Set:
		parts := []string{}
		for _, element := range value.Elements() {
			parts = append(parts, attributeValueString(element))
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
