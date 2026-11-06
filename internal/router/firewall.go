package router

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type firewallChain struct {
	table string
	name  string
}

type firewallRules struct {
	chains   map[firewallChain][][]string
	policies map[firewallChain]string
}

var stockDNATChains = map[string]bool{
	"FORWARD": true, "zone_lan_forward": true, "zone_guest_forward": true,
	"zone_iot_forward": true, "zone_nas_forward": true, "zone_visitor_forward": true,
	"zone_wan_forward": true,
}

var firewallProtocols = map[string]string{
	"tcp": "6", "udp": "17", "icmp": "1", "ipv6-icmp": "58", "icmpv6": "58",
	"igmp": "2", "esp": "50", "ah": "51", "gre": "47",
}

func firewallMetric(value string) string {
	if protocol, ok := firewallProtocols[value]; ok {
		return protocol
	}
	if number, err := strconv.Atoi(value); err == nil {
		return strconv.Itoa(number)
	}
	return value
}

func normalizeNetwork(value string) string {
	if !strings.Contains(value, "/") {
		if ip := net.ParseIP(value); ip != nil {
			if ip.To4() != nil {
				return ip.String() + "/32"
			}
			return ip.String() + "/128"
		}
		return value
	}
	parts := strings.SplitN(value, "/", 2)
	ip := net.ParseIP(parts[0])
	if ip == nil {
		return value
	}
	ones, err := strconv.Atoi(parts[1])
	if err != nil {
		maskIP := net.ParseIP(parts[1])
		if maskIP == nil || maskIP.To4() == nil {
			return value
		}
		mask := net.IPMask(maskIP.To4())
		ones, _ = mask.Size()
	}
	if ip.To4() != nil {
		return ip.String() + "/" + strconv.Itoa(ones)
	}
	return value
}

func normalizeFirewallRule(arguments []string) []string {
	leading := make([][2]string, 0, 3)
	rest := make([]string, 0, len(arguments))
	negated := false
	for _, value := range arguments {
		if value == "!" {
			negated = true
		}
	}
	for index := 0; index < len(arguments); index++ {
		key := arguments[index]
		if index+1 < len(arguments) && key == "-m" && (arguments[index+1] == "tcp" || arguments[index+1] == "udp" || arguments[index+1] == "icmp" || arguments[index+1] == "comment") {
			index++
			continue
		}
		if key == "--comment" {
			index++
			continue
		}
		if (key == "-s" || key == "-d" || key == "-p") && index+1 < len(arguments) && !negated {
			value := arguments[index+1]
			if key == "-p" {
				leading = append(leading, [2]string{key, firewallMetric(value)})
			} else {
				leading = append(leading, [2]string{key, normalizeNetwork(value)})
			}
			index++
			continue
		}
		rest = append(rest, key)
	}
	// iptables treats the leading source, destination and protocol matches as a
	// set. Sorting them makes generated and running rules comparable even when
	// the two writers emit them in a different order.
	for i := 1; i < len(leading); i++ {
		for j := i; j > 0 && leading[j][0] < leading[j-1][0]; j-- {
			leading[j], leading[j-1] = leading[j-1], leading[j]
		}
	}
	result := make([]string, 0, len(arguments))
	for _, pair := range leading {
		result = append(result, pair[0], pair[1])
	}
	return append(result, rest...)
}

func parseFirewall(source string) (firewallRules, error) {
	result := firewallRules{chains: map[firewallChain][][]string{}, policies: map[firewallChain]string{}}
	table := ""
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || line == "COMMIT" {
			continue
		}
		if strings.HasPrefix(line, "*") {
			table = strings.TrimPrefix(line, "*")
			continue
		}
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}
		current := table
		if args[0] == "iptables" {
			args = args[1:]
			current = "filter"
			for index := 0; index < len(args); index++ {
				if args[index] == "-t" && index+1 < len(args) {
					current = args[index+1]
					args = append(args[:index], args[index+2:]...)
					break
				}
			}
		}
		if len(args) == 0 {
			continue
		}
		if strings.HasPrefix(args[0], ":") {
			chain := firewallChain{table: current, name: strings.TrimPrefix(args[0], ":")}
			if _, ok := result.chains[chain]; !ok {
				result.chains[chain] = nil
			}
			if len(args) > 1 {
				result.policies[chain] = args[1]
			}
			continue
		}
		if len(args) < 2 {
			return firewallRules{}, errors.New("Unsupported firewall rule output.")
		}
		switch args[0] {
		case "-N":
			chain := firewallChain{table: current, name: args[1]}
			if _, ok := result.chains[chain]; !ok {
				result.chains[chain] = nil
			}
		case "-P":
			if len(args) < 3 {
				return firewallRules{}, errors.New("Unsupported firewall policy output.")
			}
			result.policies[firewallChain{table: current, name: args[1]}] = args[2]
		case "-A":
			if len(args) < 3 {
				return firewallRules{}, errors.New("Unsupported firewall rule output.")
			}
			chain := firewallChain{table: current, name: args[1]}
			result.chains[chain] = append(result.chains[chain], normalizeFirewallRule(args[2:]))
		}
	}
	return result, nil
}

func ruleTarget(rule []string) string {
	for index, value := range rule {
		if value == "-j" && index+1 < len(rule) {
			return rule[index+1]
		}
	}
	return ""
}

func hasJump(rule []string) bool {
	for _, value := range rule {
		if value == "-j" || value == "--jump" || value == "-g" || value == "--goto" {
			return true
		}
	}
	return false
}

func containsRule(rule []string, values ...string) bool {
	for _, value := range values {
		for _, item := range rule {
			if item == value {
				return true
			}
		}
	}
	return false
}

func verifyRuntimeFirewall(generated, running string) error {
	expected, err := parseFirewall(generated)
	if err != nil {
		return err
	}
	actual, err := parseFirewall(running)
	if err != nil {
		return err
	}
	if expected.policies[firewallChain{table: "filter", name: "FORWARD"}] != "DROP" || actual.policies[firewallChain{table: "filter", name: "FORWARD"}] != "DROP" {
		return errors.New("Cannot verify the FORWARD default policy.")
	}
	if expected.policies[firewallChain{table: "filter", name: "INPUT"}] == "" || actual.policies[firewallChain{table: "filter", name: "INPUT"}] != expected.policies[firewallChain{table: "filter", name: "INPUT"}] {
		return errors.New("The INPUT default policy differs from the generated policy.")
	}
	if err := verifyTable(expected, actual, "filter", "INPUT", "FORWARD"); err != nil {
		return err
	}
	return verifyTable(expected, actual, "nat", "PREROUTING")
}

func verifyTable(expected, actual firewallRules, table string, roots ...string) error {
	pending := append([]string{}, roots...)
	seen := map[string]bool{}
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[name] {
			continue
		}
		seen[name] = true
		chain := firewallChain{table: table, name: name}
		desired := expected.chains[chain]
		position := 0
		for _, rule := range actual.chains[chain] {
			target := ruleTarget(rule)
			child, childExists := actual.chains[firewallChain{table: table, name: target}]
			if target != "" && childExists {
				pending = append(pending, target)
			}
			if position < len(desired) && equalRule(rule, desired[position]) {
				position++
				continue
			}
			counterOnly := !containsRule(rule, "-g", "--goto", "--jump") && target == ""
			harmlessChain := target != "" && childExists && allRulesHarmless(child)
			stockDNAT := table == "filter" && stockDNATChains[name] && position == 0 && equalRule(rule, []string{"-m", "conntrack", "--ctstate", "DNAT", "-j", "ACCEPT"})
			stockICMP := table == "filter" && name == "INPUT" && equalRule(rule, []string{"-p", "1", "-i", "br-lan", "--icmp-type", "8", "-m", "limit", "--limit", "25/sec", "-j", "ACCEPT"})
			if !(counterOnly || harmlessChain || stockDNAT || stockICMP || target == "DROP" || target == "REJECT" || target == "LOG") {
				return fmt.Errorf("The running %s table contains an undeclared rule: %s", table, name)
			}
		}
		if position != len(desired) {
			return fmt.Errorf("Generated rules are missing or out of order in the %s table: %s", table, name)
		}
	}
	return nil
}

func equalRule(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func allRulesHarmless(rules [][]string) bool {
	for _, rule := range rules {
		if hasJump(rule) {
			return false
		}
	}
	return true
}
