package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSambaShareSpecRoundTrip(t *testing.T) {
	spec := NewSambaShareResource().(*typedResource).spec
	values := map[string]attr.Value{
		"section": types.StringValue("share1"), "share_name": types.StringValue("media"), "path": types.StringValue("/mnt/media"),
		"users": types.StringValue("admin"), "read_only": types.StringValue("no"), "writeable": types.StringValue("yes"),
		"browseable": types.StringValue("yes"), "guest_ok": types.StringValue("no"), "guest_only": types.StringValue("no"),
		"inherit_owner": types.StringValue(""), "create_mask": types.StringValue("0666"), "dir_mask": types.StringValue("0777"),
		"write_list": types.StringValue(""), "read_list": types.StringValue(""), "vfs_objects": types.StringValue(""),
		"force_root": types.BoolValue(false), "timemachine": types.BoolValue(false), "timemachine_maxsize": types.StringValue(""),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if section.Package != "samba4" || section.Type != "sambashare" || section.Options["name"] != "media" {
		t.Fatalf("unexpected share: %+v", section)
	}
	loaded := spec.Load(section)
	if loaded["path"].(types.String).ValueString() != "/mnt/media" {
		t.Fatalf("unexpected round trip: %+v", loaded)
	}
}

func TestClientFilterListSpec(t *testing.T) {
	spec := NewClientFilterListResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"name": types.StringValue("black"), "macs": stringSet([]string{"aa:bb:cc:dd:ee:ff"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "list" || section.Lists["mac"][0] != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected filter list: %+v", section)
	}
	if _, err := spec.Selector(map[string]attr.Value{"name": types.StringValue("grey"), "macs": stringSet(nil)}); err == nil {
		t.Fatal("invalid list name was accepted")
	}
}

func TestNetworkIsolationSpec(t *testing.T) {
	spec := NewNetworkIsolationResource().(*typedResource).spec
	section, err := spec.Selector(map[string]attr.Value{
		"network": types.StringValue("guest"), "ap_isolate": types.BoolValue(true),
		"wan_isolate": types.BoolValue(false), "transfer_enable": types.BoolValue(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if section.Type != "secure_option" || section.Options["ap_isolate"] != "1" {
		t.Fatalf("unexpected isolation: %+v", section)
	}
}
