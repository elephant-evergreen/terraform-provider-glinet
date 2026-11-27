package provider

import "testing"

func TestCompositeImportIdentities(t *testing.T) {
	cases := []struct {
		resource *typedResource
		id       string
		kind     string
		name     string
	}{
		{NewQoSInterfaceResource().(*typedResource), "wan:@wan[0]", "wan", "@wan[0]"},
		{NewRoutePolicyRuleResource().(*typedResource), "default:gl_process", "default", "gl_process"},
	}
	for _, tc := range cases {
		values, err := tc.resource.spec.Import(tc.id)
		if err != nil {
			t.Fatalf("%s import: %v", tc.resource.spec.TypeName, err)
		}
		if stringOf(values, "kind") != tc.kind || stringOf(values, "name") != tc.name {
			t.Fatalf("%s imported kind=%q name=%q", tc.resource.spec.TypeName, stringOf(values, "kind"), stringOf(values, "name"))
		}
		id, err := tc.resource.spec.ID(values)
		if err != nil || id != tc.id {
			t.Fatalf("%s identifier %q, want %q (%v)", tc.resource.spec.TypeName, id, tc.id, err)
		}
		if _, err := tc.resource.spec.Import("missing-separator"); err == nil {
			t.Fatalf("%s accepted an invalid import ID", tc.resource.spec.TypeName)
		}
	}
}
