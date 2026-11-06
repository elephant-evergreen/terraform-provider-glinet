package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFirewallRuleMultiAddressSpec(t *testing.T) {
	spec := typedFirewallRuleSpec()
	values := map[string]attr.Value{
		"name": types.StringValue("Home-IaC-Block-WAN-Subnets"),
		"src":  types.StringValue("*"), "dest": types.StringValue("wan"),
		"src_ips": stringSet(nil), "dest_ips": stringSet([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}),
		"protocols": stringSet([]string{"all"}), "src_ports": stringSet(nil), "dest_ports": stringSet(nil),
		"family": types.StringValue("ipv4"), "target": types.StringValue("REJECT"), "enabled": types.BoolValue(true), "position": types.Int64Value(37),
	}
	section, err := spec.Selector(values)
	if err != nil {
		t.Fatal(err)
	}
	if len(section.Lists["dest_ip"]) != 3 || section.Options["dest_ip"] != "" {
		t.Fatalf("dest_ip list was not preserved: %+v", section)
	}
	loaded := spec.Load(section)
	if len(loaded["dest_ips"].(types.Set).Elements()) != 3 {
		t.Fatalf("dest_ips did not round trip: %+v", loaded["dest_ips"])
	}
	single := map[string]attr.Value{
		"name": types.StringValue("Home-IaC-Admin-Agent"),
		"src":  types.StringValue("lan"), "dest": types.StringValue("iot"),
		"src_ips": stringSet([]string{"192.168.8.237"}), "dest_ips": stringSet(nil),
		"protocols": stringSet([]string{"tcp"}), "src_ports": stringSet(nil), "dest_ports": stringSet([]string{"22"}),
		"family": types.StringValue("ipv4"), "target": types.StringValue("ACCEPT"), "enabled": types.BoolValue(true), "position": types.Int64Value(41),
	}
	section, err = spec.Selector(single)
	if err != nil {
		t.Fatal(err)
	}
	if section.Options["src_ip"] != "192.168.8.237" || len(section.Lists["src_ip"]) != 0 {
		t.Fatalf("single src_ip was not scalar: %+v", section)
	}
}
