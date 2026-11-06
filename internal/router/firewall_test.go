package router

import (
	"strings"
	"testing"
)

const generatedFirewall = `iptables -t filter -P INPUT ACCEPT
iptables -t filter -P FORWARD DROP
iptables -t filter -P OUTPUT ACCEPT
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
:OUTPUT ACCEPT [0:0]
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
:INPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
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

func TestRuntimeFirewallAllowsOnlyCounterOnlyChainTrees(t *testing.T) {
	for _, table := range []string{"filter", "nat"} {
		root := "OUTPUT"
		baseline := strings.Replace(runningFirewall, "*"+table+"\n", "*"+table+"\n:dispatcher - [0:0]\n:leaf - [0:0]\n", 1)
		baseline = appendRunningFirewallRules(t, baseline, table, "-A "+root+" -p tcp --dport 53 -j dispatcher", "-A dispatcher -j leaf")
		if err := verifyRuntimeFirewall(generatedFirewall, baseline); err != nil {
			t.Fatalf("counter-only %s chain tree was rejected: %v", table, err)
		}
		target := "ACCEPT"
		if table == "nat" {
			target = "DNAT --to-destination 192.168.11.10:53"
		}
		for _, rule := range []string{"-A leaf -j " + target, "-A leaf -j dispatcher", "-A leaf -g dispatcher"} {
			running := appendRunningFirewallRules(t, baseline, table, rule)
			if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil {
				t.Fatalf("unsafe %s chain tree was accepted: %s", table, rule)
			}
		}
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
		"-A INPUT -m conntrack --ctstate DNAT -j ACCEPT",
		"-A INPUT -j forwarding_rule\n-A forwarding_rule -j ACCEPT",
	} {
		running := strings.Replace(runningFirewall, "-A FORWARD -j parental_control", extra+"\n-A FORWARD -j parental_control", 1)
		if err := verifyRuntimeFirewall(generatedFirewall, running); err == nil {
			t.Fatal("Unexpected INPUT ACCEPT was accepted")
		}
	}
}

func TestRuntimeFirewallRejectsUndeclaredNAT(t *testing.T) {
	for _, chain := range []string{"PREROUTING", "INPUT", "POSTROUTING", "OUTPUT"} {
		for _, target := range []string{
			"DNAT --to-destination 192.168.11.10:22",
			"REDIRECT --to-ports 22",
			"NETMAP --to 192.168.11.0/24",
		} {
			if (chain == "INPUT" || chain == "POSTROUTING") && !strings.HasPrefix(target, "NETMAP ") {
				continue
			}
			t.Run(chain+"/"+target, func(t *testing.T) {
				running := appendRunningFirewallRules(t, runningFirewall, "nat", "-A "+chain+" -p tcp --dport 9999 -j "+target)
				err := verifyRuntimeFirewall(generatedFirewall, running)
				if err == nil || !strings.Contains(err.Error(), "nat table") {
					t.Fatalf("Expected a nat table rejection for %s: %v", target, err)
				}
			})
		}
	}
}

func TestRuntimeFirewallAcceptsDeclaredDestinationNAT(t *testing.T) {
	for _, chain := range []string{"PREROUTING", "OUTPUT"} {
		redirect := "-A " + chain + " -p tcp -m tcp --dport 8080 -j DNAT --to-destination 192.168.8.50:80"
		generated := generatedFirewall + "iptables -t nat " + redirect + "\n"
		running := appendRunningFirewallRules(t, runningFirewall, "nat", redirect)
		if err := verifyRuntimeFirewall(generated, running); err != nil {
			t.Fatalf("declared redirect in %s was rejected: %v", chain, err)
		}
		if err := verifyRuntimeFirewall(generated, runningFirewall); err == nil {
			t.Fatalf("a generated redirect missing from %s was accepted", chain)
		}
	}
}

func TestRuntimeFirewallVerifiesBuiltinChains(t *testing.T) {
	for _, test := range []struct {
		table  string
		root   string
		target string
	}{
		{"filter", "INPUT", "ACCEPT"},
		{"filter", "FORWARD", "ACCEPT"},
		{"filter", "OUTPUT", "ACCEPT"},
		{"nat", "PREROUTING", "NETMAP --to 192.168.11.0/24"},
		{"nat", "INPUT", "NETMAP --to 192.168.11.0/24"},
		{"nat", "OUTPUT", "NETMAP --to 192.168.11.0/24"},
		{"nat", "POSTROUTING", "NETMAP --to 192.168.11.0/24"},
	} {
		for _, nested := range []bool{false, true} {
			chain := test.root
			generated := generatedFirewall
			running := runningFirewall
			if nested {
				chain = "verification_child"
				generated += "iptables -t " + test.table + " -N " + chain + "\niptables -t " + test.table + " -A " + test.root + " -j " + chain + "\n"
				running = strings.Replace(running, "*"+test.table+"\n", "*"+test.table+"\n:"+chain+" - [0:0]\n", 1)
				running = appendRunningFirewallRules(t, running, test.table, "-A "+test.root+" -j "+chain)
			}
			rule := "-A " + chain + " -p tcp -m tcp --dport 8080 -j " + test.target
			other := strings.Replace(rule, "8080", "8081", 1)
			declared := generated + "iptables -t " + test.table + " " + rule + "\n"
			matching := appendRunningFirewallRules(t, running, test.table, rule)
			for _, scenario := range []struct {
				name      string
				generated string
				running   string
				wantError string
			}{
				{"declared", declared, matching, ""},
				{"undeclared", generated, matching, "The running " + test.table + " table contains an undeclared rule: " + chain},
				{"missing", declared, running, "Generated rules are missing or out of order in the " + test.table + " table: " + chain},
				{"changed", declared, appendRunningFirewallRules(t, running, test.table, other), "The running " + test.table + " table contains an undeclared rule: " + chain},
				{"reordered", declared + "iptables -t " + test.table + " " + other + "\n", appendRunningFirewallRules(t, running, test.table, other, rule), "The running " + test.table + " table contains an undeclared rule: " + chain},
			} {
				name := test.table + "/" + test.root + "/" + chain + "/" + scenario.name
				t.Run(name, func(t *testing.T) {
					err := verifyRuntimeFirewall(scenario.generated, scenario.running)
					if scenario.wantError == "" {
						if err != nil {
							t.Fatalf("declared rule was rejected: %v", err)
						}
					} else if err == nil || err.Error() != scenario.wantError {
						t.Fatalf("expected %q, got %v", scenario.wantError, err)
					}
				})
			}
		}
	}
}

func TestRuntimeFirewallRejectsChangedDefaultPolicies(t *testing.T) {
	for _, test := range []struct {
		chain   string
		policy  string
		changed string
	}{
		{"INPUT", "ACCEPT", "DROP"},
		{"FORWARD", "DROP", "ACCEPT"},
		{"OUTPUT", "ACCEPT", "DROP"},
	} {
		for _, policy := range []string{test.changed, ""} {
			running := strings.Replace(runningFirewall, ":"+test.chain+" "+test.policy+" [0:0]", ":"+test.chain+" "+policy+" [0:0]", 1)
			generated := generatedFirewall
			if policy == "" {
				running = strings.Replace(runningFirewall, ":"+test.chain+" "+test.policy+" [0:0]\n", "", 1)
				generated = strings.Replace(generatedFirewall, "iptables -t filter -P "+test.chain+" "+test.policy+"\n", "", 1)
			}
			for _, source := range []struct {
				name      string
				generated string
				running   string
			}{
				{"running", generatedFirewall, running},
				{"generated", generated, runningFirewall},
			} {
				if policy != "" && source.name == "generated" {
					continue
				}
				t.Run(test.chain+"/"+policy+"/"+source.name, func(t *testing.T) {
					if err := verifyRuntimeFirewall(source.generated, source.running); err == nil || !strings.Contains(err.Error(), "default policy") {
						t.Fatalf("expected a default policy rejection for %s, got %v", test.chain, err)
					}
				})
			}
		}
	}
}

func appendRunningFirewallRules(t *testing.T, source, table string, rules ...string) string {
	t.Helper()
	start := strings.Index(source, "*"+table+"\n")
	if start < 0 {
		t.Fatalf("missing %s table in fixture", table)
	}
	end := strings.Index(source[start:], "COMMIT\n")
	if end < 0 {
		t.Fatalf("missing %s table commit in fixture", table)
	}
	position := start + end
	return source[:position] + strings.Join(rules, "\n") + "\n" + source[position:]
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
