package router

import "testing"

func TestNetworkDisruptionRequired(t *testing.T) {
	for _, test := range []struct {
		name string
		plan reloadPlan
		want bool
	}{
		{"empty", reloadPlan{}, false},
		{"firewall", reloadPlan{Firewall: true}, false},
		{"dns and dhcp", reloadPlan{DNS: true, DHCP: true}, false},
		{"network reload", reloadPlan{Reboot: true}, true},
		{"wireless reload", reloadPlan{Wireless: true}, true},
		{"cloud switch", reloadPlan{Cloud: true}, true},
		{"network and firewall", reloadPlan{Reboot: true, Firewall: true}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := networkDisruptionRequired(test.plan); got != test.want {
				t.Fatalf("networkDisruptionRequired(%+v) = %v, want %v", test.plan, got, test.want)
			}
		})
	}
}
