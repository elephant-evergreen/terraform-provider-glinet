package provider

import (
	"context"
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWirelessStatePreservesKeyVersion(t *testing.T) {
	r := newTypedResource(typedWirelessInterfaceSpec()).(*typedResource)
	var schema frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &schema)
	actual := router.Section{Package: "wireless", Type: "wifi-iface", Name: "wifi2g", Options: map[string]string{"ssid": "Example", "mode": "ap", "encryption": "psk2", "disabled": "0", "isolate": "0"}}
	planned := map[string]attr.Value{
		"section":     types.StringValue("wifi2g"),
		"key_wo":      types.StringNull(),
		"key_version": types.Int64Value(3),
	}
	object, diagnostics := r.stateObject(planned, actual, "")
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if object.Attributes()["key_version"].(types.Int64).ValueInt64() != 3 {
		t.Fatalf("key_version was overwritten: %+v", object.Attributes()["key_version"])
	}
}

func TestWirelessSecurityValidation(t *testing.T) {
	intPtr := func(value int64) *int64 { return &value }
	boolPtr := func(value bool) *bool { return &value }
	for _, tc := range []struct {
		name       string
		encryption string
		pmf        *int64
		wps        *bool
		wantError  bool
	}{
		{name: "sae requires required PMF", encryption: "sae", pmf: intPtr(2), wps: boolPtr(false)},
		{name: "sae rejects disabled PMF", encryption: "sae", pmf: intPtr(0), wps: boolPtr(false), wantError: true},
		{name: "sae rejects WPS", encryption: "sae", pmf: intPtr(2), wps: boolPtr(true), wantError: true},
		{name: "mixed accepts optional PMF", encryption: "sae-mixed", pmf: intPtr(1), wps: boolPtr(false)},
		{name: "mixed rejects disabled PMF", encryption: "sae-mixed", pmf: intPtr(0), wps: boolPtr(false), wantError: true},
		{name: "psk2 accepts WPS", encryption: "psk2", pmf: intPtr(0), wps: boolPtr(true)},
		{name: "open rejects WPS", encryption: "none", pmf: intPtr(0), wps: boolPtr(true), wantError: true},
		{name: "unknown mode", encryption: "wpa4", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]attr.Value{"encryption": stringValue(tc.encryption), "ieee80211w": types.Int64Null(), "wps_pushbutton": types.BoolNull()}
			if tc.pmf != nil {
				values["ieee80211w"] = int64Value(*tc.pmf)
			}
			if tc.wps != nil {
				values["wps_pushbutton"] = boolValue(*tc.wps)
			}
			err := validateWirelessSecurity(values)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateWirelessSecurity() = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

func TestWirelessPlanDefaults(t *testing.T) {
	config := map[string]attr.Value{"ieee80211w": types.Int64Null(), "wps_pushbutton": types.BoolNull()}
	plan := map[string]attr.Value{"encryption": stringValue("sae"), "ieee80211w": types.Int64Null(), "wps_pushbutton": types.BoolNull()}
	updated, err := modifyWirelessPlan(plan, config, map[string]attr.Value{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if updated["ieee80211w"].(types.Int64).ValueInt64() != 2 || updated["wps_pushbutton"].(types.Bool).ValueBool() {
		t.Fatalf("unexpected WPA3 defaults: %+v", updated)
	}

	plan = map[string]attr.Value{"encryption": stringValue("sae"), "ieee80211w": int64Value(0), "wps_pushbutton": boolValue(true)}
	state := map[string]attr.Value{"encryption": stringValue("psk2"), "ieee80211w": int64Value(0), "wps_pushbutton": boolValue(true)}
	updated, err = modifyWirelessPlan(plan, config, state, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated["ieee80211w"].(types.Int64).ValueInt64() != 2 || updated["wps_pushbutton"].(types.Bool).ValueBool() {
		t.Fatalf("WPA3 change did not reset PMF and WPS: %+v", updated)
	}

	plan = map[string]attr.Value{"encryption": stringValue("psk2"), "ieee80211w": int64Value(0), "wps_pushbutton": boolValue(true)}
	state = map[string]attr.Value{"encryption": stringValue("psk2"), "ieee80211w": int64Value(0), "wps_pushbutton": boolValue(true)}
	updated, err = modifyWirelessPlan(plan, config, state, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated["ieee80211w"].(types.Int64).ValueInt64() != 0 || !updated["wps_pushbutton"].(types.Bool).ValueBool() {
		t.Fatalf("WPA2 update changed existing WPS: %+v", updated)
	}

	plan = map[string]attr.Value{"encryption": stringValue("psk2"), "ieee80211w": int64Value(2), "wps_pushbutton": boolValue(false)}
	state = map[string]attr.Value{"encryption": stringValue("psk2"), "ieee80211w": int64Value(2), "wps_pushbutton": boolValue(false)}
	updated, err = modifyWirelessPlan(plan, config, state, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated["ieee80211w"].(types.Int64).ValueInt64() != 2 {
		t.Fatalf("unchanged PMF setting was reset: %+v", updated)
	}
}

func TestWirelessEncryptionValidator(t *testing.T) {
	v := wirelessEncryptionValidator{}
	request := validator.StringRequest{ConfigValue: types.StringValue("psk"), Path: path.Root("encryption")}
	response := validator.StringResponse{}
	v.ValidateString(context.Background(), request, &response)
	if len(response.Diagnostics.Warnings()) != 1 || response.Diagnostics.HasError() {
		t.Fatalf("deprecated mode = %+v", response.Diagnostics)
	}

	request.ConfigValue = types.StringValue("wpa4")
	response = validator.StringResponse{}
	v.ValidateString(context.Background(), request, &response)
	if len(response.Diagnostics.Errors()) != 1 {
		t.Fatalf("invalid mode = %+v", response.Diagnostics)
	}
}

func TestWirelessLoadSecurity(t *testing.T) {
	spec := typedWirelessInterfaceSpec()
	loaded := spec.Load(router.Section{Package: "wireless", Type: "wifi-iface", Name: "wifi2g", Options: map[string]string{"encryption": "sae"}})
	if loaded["ieee80211w"].(types.Int64).ValueInt64() != 2 || loaded["wps_pushbutton"].(types.Bool).ValueBool() {
		t.Fatalf("derived defaults = %+v", loaded)
	}
	loaded = spec.Load(router.Section{Package: "wireless", Type: "wifi-iface", Name: "wifi2g", Options: map[string]string{"encryption": "sae-mixed", "ieee80211w": "1", "wps_pushbutton": "1"}})
	if loaded["ieee80211w"].(types.Int64).ValueInt64() != 1 || !loaded["wps_pushbutton"].(types.Bool).ValueBool() {
		t.Fatalf("loaded security = %+v", loaded)
	}
}
