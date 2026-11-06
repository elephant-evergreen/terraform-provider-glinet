package provider

import (
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringOf(values map[string]attr.Value, name string) string {
	value, ok := values[name].(types.String)
	if !ok || value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func boolOf(values map[string]attr.Value, name string, fallback bool) bool {
	value, ok := values[name].(types.Bool)
	if !ok || value.IsNull() || value.IsUnknown() {
		return fallback
	}
	return value.ValueBool()
}

func int64Of(values map[string]attr.Value, name string) int64 {
	value, ok := values[name].(types.Int64)
	if !ok || value.IsNull() || value.IsUnknown() {
		return 0
	}
	return value.ValueInt64()
}

func stringsOf(values map[string]attr.Value, name string) []string {
	value, ok := values[name].(types.Set)
	if !ok || value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := []string{}
	for _, element := range value.Elements() {
		if text, ok := element.(types.String); ok && !text.IsNull() && !text.IsUnknown() {
			result = append(result, text.ValueString())
		}
	}
	sort.Strings(result)
	return result
}

func stringValue(value string) types.String { return types.StringValue(value) }
func boolValue(value bool) types.Bool       { return types.BoolValue(value) }
func int64Value(value int64) types.Int64    { return types.Int64Value(value) }

func stringSet(values []string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.SetValueMust(types.StringType, elements)
}

func nullValue(valueType attr.Type) attr.Value {
	switch valueType {
	case types.StringType:
		return types.StringNull()
	case types.BoolType:
		return types.BoolNull()
	case types.Int64Type:
		return types.Int64Null()
	}
	if setType, ok := valueType.(types.SetType); ok {
		return types.SetNull(setType.ElemType)
	}
	return types.StringNull()
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func joinStrings(values []string) string { return strings.Join(values, " ") }

func splitFields(value string) []string { return strings.Fields(value) }
