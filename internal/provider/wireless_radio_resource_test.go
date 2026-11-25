package provider

import (
	"testing"

	"github.com/elephant-evergreen/terraform-provider-glinet/internal/router"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWirelessRadioLoad(t *testing.T) {
	spec := typedWirelessRadioSpec()
	loaded := spec.Load(router.Section{Package: "wireless", Type: "wifi-device", Name: "mt798611", Options: map[string]string{
		"band": "2g", "channel": "auto", "htmode": "HE40", "txpower": "100", "country": "JP", "disabled": "0",
	}})
	if loaded["device"].(types.String).ValueString() != "mt798611" || loaded["band"].(types.String).ValueString() != "2g" {
		t.Fatalf("identity = %+v", loaded)
	}
	if loaded["channel"].(types.String).ValueString() != "auto" || loaded["htmode"].(types.String).ValueString() != "HE40" {
		t.Fatalf("radio settings = %+v", loaded)
	}
	if loaded["txpower"].(types.Int64).ValueInt64() != 100 || loaded["country"].(types.String).ValueString() != "JP" || loaded["disabled"].(types.Bool).ValueBool() {
		t.Fatalf("radio values = %+v", loaded)
	}
}

func TestWirelessRadioValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		device    string
		band      string
		channel   string
		htmode    string
		txpower   int64
		country   string
		wantError bool
	}{
		{name: "2g auto", device: "mt798611", band: "2g", channel: "auto", htmode: "HE40", txpower: 100, country: "JP"},
		{name: "5g channel", device: "mt798612", band: "5g", channel: "36", htmode: "HE80"},
		{name: "2g rejects 5g channel", device: "mt798611", band: "2g", channel: "36", wantError: true},
		{name: "5g rejects 2g width", device: "mt798612", band: "5g", channel: "auto", htmode: "HT40", wantError: true},
		{name: "invalid channel", device: "mt798611", channel: "197", wantError: true},
		{name: "invalid htmode", device: "mt798611", channel: "auto", htmode: "BE320", wantError: true},
		{name: "invalid txpower", device: "mt798611", channel: "auto", txpower: 101, wantError: true},
		{name: "invalid country", device: "mt798611", channel: "auto", country: "jp", wantError: true},
		{name: "missing device", channel: "auto", wantError: true},
		{name: "missing channel", device: "mt798611", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]attr.Value{
				"device": stringValue(tc.device), "band": stringValue(tc.band), "channel": stringValue(tc.channel),
				"htmode": stringValue(tc.htmode), "txpower": int64Value(tc.txpower), "country": stringValue(tc.country),
			}
			err := validateWirelessRadio(values)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateWirelessRadio() = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}

func TestWirelessRadioSelectorOmitsUnsetValues(t *testing.T) {
	spec := typedWirelessRadioSpec()
	section, err := spec.Selector(map[string]attr.Value{
		"device":   stringValue("mt798611"),
		"channel":  stringValue("auto"),
		"htmode":   types.StringNull(),
		"txpower":  types.Int64Null(),
		"country":  types.StringNull(),
		"disabled": types.BoolNull(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(section.Options) != 1 || section.Options["channel"] != "auto" {
		t.Fatalf("selector options = %+v", section.Options)
	}
}

func TestWirelessRadioImport(t *testing.T) {
	spec := typedWirelessRadioSpec()
	values, err := spec.Import("mt798611")
	if err != nil {
		t.Fatal(err)
	}
	id, err := spec.ID(values)
	if err != nil || id != "mt798611" {
		t.Fatalf("import ID = %q, %v", id, err)
	}
}
