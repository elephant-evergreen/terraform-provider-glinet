package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDPISpecRoundTrip(t *testing.T) {
	spec := NewDPIResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"log_level": types.StringValue("INFO"), "expire_time": types.Int64Value(0),
		"library_auto_update": types.BoolValue(false), "ota_auto_update": types.BoolValue(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl_dpi" || section.Type != "gl_dpi" || section.Options["log"] != "INFO" {
		t.Fatalf("unexpected DPI section: %+v", section)
	}
}

func TestContentProtectionSpecRoundTrip(t *testing.T) {
	spec := NewContentProtectionResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "categories": stringSet([]string{"adult", "malware"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(section.Lists["category"]) != 2 || section.Options["enabled"] != "1" {
		t.Fatalf("unexpected content protection: %+v", section)
	}
}

func TestNATAndAccelerationAndLEDSpecs(t *testing.T) {
	nat := NewNATResource().(*typedResource).spec
	natSection, err := nat.Selector(map[string]attr.Value{"full_cone": types.BoolValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if natSection.Package != "fullconenat" || natSection.Options["enable"] != "1" {
		t.Fatalf("unexpected NAT: %+v", natSection)
	}

	accel := NewHardwareAccelerationResource().(*typedResource).spec
	accelSection, err := accel.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "hqos": types.BoolValue(false),
		"txq_num": types.Int64Value(16), "scheduling": types.StringValue("wrr"), "acc_type": types.StringValue("1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if accelSection.Package != "mtkhnat" || accelSection.Options["txq_num"] != "16" {
		t.Fatalf("unexpected acceleration: %+v", accelSection)
	}

	led := NewLEDResource().(*typedResource).spec
	ledSection, err := led.Selector(map[string]attr.Value{"enabled": types.BoolValue(true), "daemon": types.BoolValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if ledSection.Package != "gl_led" || ledSection.Options["led_enable"] != "1" {
		t.Fatalf("unexpected LED: %+v", ledSection)
	}
}
