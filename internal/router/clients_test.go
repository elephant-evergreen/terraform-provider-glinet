package router

import "testing"

func TestParseLeases(t *testing.T) {
	leases, err := parseLeases("43200 ea:31:03:53:68:ed 192.168.8.192 iPad 01:ea:31:03:53:68:ed\n43200 aa:bb:cc:dd:ee:ff 192.168.9.2 anonymous *\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 2 {
		t.Fatalf("leases=%d", len(leases))
	}
	if leases[0].Hostname != "iPad" || leases[0].ClientID != "01:ea:31:03:53:68:ed" || leases[0].LeaseSeconds != 43200 {
		t.Fatalf("unexpected first lease: %+v", leases[0])
	}
	if leases[1].ClientID != "" {
		t.Fatalf("wildcard client id must be empty: %+v", leases[1])
	}
}

func TestParseLeasesRejectsMalformed(t *testing.T) {
	if _, err := parseLeases("not-a-number aa:bb 192.168.1.2 host\n"); err == nil {
		t.Fatal("malformed lease was accepted")
	}
}
