package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestTimerSpecRoundTrip(t *testing.T) {
	spec := NewTimerResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "band_2g_func": types.StringValue("turn_onoff"),
		"band_5g_func": types.StringValue("turn_onoff"), "band_6g_func": types.StringValue("turn_onoff"),
		"mld_func": types.StringValue("turn_onoff"),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl_timer" || section.Options["2G_func"] != "turn_onoff" {
		t.Fatalf("unexpected timer section: %+v", section)
	}
}

func TestWiFiTimerSpecRoundTrip(t *testing.T) {
	spec := NewWiFiTimerResource().(*typedResource).spec
	values := map[string]attr.Value{
		"name": types.StringValue("5gwifi"), "enabled": types.BoolValue(true),
		"func": types.StringValue("turn_onoff"), "band": types.StringValue("5G"),
		"guest": types.BoolValue(false), "turn_on_hour": types.StringValue("07"),
		"turn_on_min": types.StringValue("00"), "turn_off_hour": types.StringValue("22"),
		"turn_off_min": types.StringValue("00"), "week": types.StringValue("0 1 2 3 4 5 6"),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl_timer" || section.Name != "5gwifi" || section.Options["turnon_hour"] != "07" {
		t.Fatalf("unexpected wifi timer: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["band"].(types.String).ValueString() != "5G" {
		t.Fatalf("unexpected wifi timer round trip: %+v", loaded)
	}
}

func TestLEDTimerAndRebootSpecs(t *testing.T) {
	led := NewLEDTimerResource().(*typedResource).spec
	ledSection, err := led.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "turn_on_hour": types.StringValue("07"),
		"turn_on_min": types.StringValue("00"), "turn_off_hour": types.StringValue("22"),
		"turn_off_min": types.StringValue("00"), "week": types.StringValue("0 1 2 3 4 5 6"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ledSection.Name != "led" || ledSection.Options["enable"] != "1" {
		t.Fatalf("unexpected LED timer: %+v", ledSection)
	}

	reboot := NewScheduledRebootResource().(*typedResource).spec
	rebootSection, err := reboot.Selector(map[string]attr.Value{
		"enabled": types.BoolValue(true), "hour": types.StringValue("03"),
		"min": types.StringValue("00"), "week": types.StringValue("0 1 2 3 4 5 6"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebootSection.Name != "reboot" || rebootSection.Options["hour"] != "03" {
		t.Fatalf("unexpected scheduled reboot: %+v", rebootSection)
	}
}
