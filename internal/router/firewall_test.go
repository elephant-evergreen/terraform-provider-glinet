package router

import (
	"strings"
	"testing"
)

const generatedFirewall = `iptables -t filter -P INPUT ACCEPT
iptables -t filter -P FORWARD DROP
iptables -t filter -N forwarding_rule
iptables -t filter -N reject
iptables -t filter -A FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
iptables -t filter -A FORWARD -p tcp -s 192.168.8.2/255.255.255.255 -d 192.168.10.254/255.255.255.255 --dport 22 -j ACCEPT
iptables -t filter -A FORWARD -j reject
iptables -t filter -A reject -j REJECT
`

const runningFirewall = `*filter
:INPUT ACCEPT [0:0]
:FORWARD DROP [0:0]
:forwarding_rule - [0:0]
:reject - [0:0]
:parental_control - [0:0]
-A FORWARD -j parental_control
-A FORWARD -m conntrack --ctstate DNAT -j ACCEPT
-A FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A FORWARD -s 192.168.8.2/32 -d 192.168.10.254/32 -p tcp -m tcp --dport 22 -j ACCEPT
-A FORWARD -j reject
-A reject -j REJECT
COMMIT
*nat
:PREROUTING ACCEPT [0:0]
COMMIT
`

func TestRuntimeFirewallAllowsStockExtensionsAndCounterOnlyChains(t *testing.T) {
	if err := verifyRuntimeFirewall(generatedFirewall, runningFirewall); err != nil {
		t.Fatal(err)
	}
	if err := verifyRuntimeFirewall(strings.ReplaceAll(generatedFirewall, "-p tcp", "-p 6"), runningFirewall); err != nil {
		t.Fatal(err)
	}
	withProbe := strings.Replace(runningFirewall, "-A reject -j REJECT", ":probe - [0:0]\n-A probe -i br-guest -m comment --comment accounting\n-A reject -j probe\n-A reject -j REJECT", 1)
	if err := verifyRuntimeFirewall(generatedFirewall, withProbe); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeFirewallRejectsUndeclaredAcceptOrBranch(t *testing.T) {
	for _, extra := range []string{
		"-A FORWARD -i br-guest -o br-vlan11 -p tcp --dport 2049 -j ACCEPT",
		"-A FORWARD -j forwarding_rule\n-A forwarding_rule -j ACCEPT",
		"-A FORWARD -g forwarding_rule",
		"-A FORWARD --goto forwarding_rule",
	} {
		t.Run(extra, func(t *testing.T) {
			running := strings.Replace(runningFirewall, "-A FORWARD -j parental_control", extra, 1)
			if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil {
				t.Fatal("Undeclared ACCEPT or branch was accepted")
			}
		})
	}
}

func TestRuntimeFirewallRejectsUnexpectedInputAccept(t *testing.T) {
	for _, extra := range []string{
		"-A INPUT -i br-guest -s 192.168.9.50/32 -p tcp --dport 22 -j ACCEPT",
		"-A INPUT -j forwarding_rule\n-A forwarding_rule -j ACCEPT",
	} {
		running := strings.Replace(runningFirewall, "-A FORWARD -j parental_control", extra+"\n-A FORWARD -j parental_control", 1)
		if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil {
			t.Fatal("Unexpected INPUT ACCEPT was accepted")
		}
	}
}

func TestRuntimeFirewallRejectsDestinationNATAndInputStateAccept(t *testing.T) {
	for _, target := range []string{
		"DNAT --to-destination 192.168.11.10:22",
		"REDIRECT --to-ports 22",
		"NETMAP --to 192.168.11.0/24",
	} {
		running := strings.Replace(runningFirewall, ":PREROUTING ACCEPT [0:0]", ":PREROUTING ACCEPT [0:0]\n-A PREROUTING -i br-guest -p tcp --dport 9999 -j "+target, 1)
		running = strings.Replace(running, "-A FORWARD -j parental_control", "-A INPUT -m conntrack --ctstate DNAT -j ACCEPT\n-A FORWARD -j parental_control", 1)
		if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil || !strings.Contains(err.Error(), "destination NAT") {
			t.Fatalf("Expected destination NAT rejection for %s: %v", target, err)
		}
	}
}

func TestRuntimeFirewallRejectsMissingRulesAndActiveDestinationNAT(t *testing.T) {
	for _, running := range []string{
		strings.Replace(runningFirewall, "-A FORWARD -j reject\n", "", 1),
		strings.Replace(strings.Replace(runningFirewall, "-A FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT\n", "", 1), "-A FORWARD -j reject", "-A FORWARD -j reject\n-A FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT", 1),
		strings.Replace(runningFirewall, ":PREROUTING ACCEPT [0:0]", ":PREROUTING ACCEPT [0:0]\n-A PREROUTING -p tcp --dport 2049 -j DNAT --to-destination 192.168.11.10", 1),
	} {
		if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil {
			t.Fatal("Missing or reordered rules were accepted")
		}
	}
}

func TestFirewallRuleNormalization(t *testing.T) {
	left := normalizeFirewallRule([]string{"-s", "192.168.8.2/255.255.255.255", "-d", "192.168.10.254/255.255.255.255", "-p", "tcp", "--dport", "22", "-j", "ACCEPT"})
	right := normalizeFirewallRule([]string{"-p", "6", "-d", "192.168.10.254/32", "-s", "192.168.8.2/32", "--dport", "22", "-j", "ACCEPT"})
	if !equalRule(left, right) {
		t.Fatalf("Normalized rules differ: %v %v", left, right)
	}
}
