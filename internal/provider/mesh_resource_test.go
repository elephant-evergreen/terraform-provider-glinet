package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMeshSpecRoundTrip(t *testing.T) {
	spec := NewMeshResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("controller"),
		"log_level": types.StringValue("DEBUG"), "log_path": types.StringValue("/var/log/gl-mesh"),
		"onboard": types.BoolValue(false), "new_join_confirmed": types.BoolValue(false),
		"ts_enabled": types.BoolValue(true), "ts_guest_vid": types.Int64Value(4002), "ts_iot_vid": types.Int64Value(4003),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "gl-mesh" || section.Type != "base" || section.Options["ts_guest_vid"] != "4002" {
		t.Fatalf("unexpected mesh section: %+v", section)
	}
}

func TestMeshValidation(t *testing.T) {
	spec := NewMeshResource().(*typedResource).spec
	values := map[string]attr.Value{
		"enabled": types.BoolValue(true), "mode": types.StringValue("bridge"),
		"log_level": types.StringValue("DEBUG"), "log_path": types.StringValue("/var/log/gl-mesh"),
		"onboard": types.BoolValue(false), "new_join_confirmed": types.BoolValue(false),
		"ts_enabled": types.BoolValue(true), "ts_guest_vid": types.Int64Value(4002), "ts_iot_vid": types.Int64Value(4003),
	}
	if err := spec.Validate(values); err == nil {
		t.Fatal("invalid mesh mode was accepted")
	}
}

func TestRepeaterSpecRoundTrip(t *testing.T) {
	spec := NewRepeaterResource().(*typedResource).spec
	values := map[string]attr.Value{
		"auto": types.BoolValue(true), "level": types.Int64Value(6), "switch_interval": types.Int64Value(30),
		"ip_wait_time": types.Int64Value(15), "blacklist_timeout": types.Int64Value(300),
		"macaddr": types.StringValue(""), "smart_reconnect": types.BoolValue(true),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "repeater" || section.Options["switch_interval"] != "30" {
		t.Fatalf("unexpected repeater section: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["blacklist_timeout"].(types.Int64).ValueInt64() != 300 {
		t.Fatalf("unexpected repeater round trip: %+v", loaded)
	}
}
