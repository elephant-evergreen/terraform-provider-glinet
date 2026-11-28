package router

import (
	"os/exec"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzShellQuote(f *testing.F) {
	for _, seed := range []string{"", "abc", "a b", "a'b", `"quoted"`, "$(echo pwned)", "`id`", "a\nb", "日本語", `\`, "*", "-n", "a;b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if strings.ContainsRune(value, 0) {
			t.Skip()
		}
		out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+quote(value)).Output()
		if err != nil {
			t.Fatalf("shell rejected %q: %v", value, err)
		}
		if string(out) != value {
			t.Fatalf("quote(%q) did not round-trip: %q", value, out)
		}
	})
}

func FuzzLuaQuote(f *testing.F) {
	for _, seed := range []string{"", "abc", "a'b", `a\b`, "a\nb", "a\rb", "日本語", "'; os.execute('true'); --"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		quoted := luaQuote(value)
		if len(quoted) < 2 || quoted[0] != '\'' || quoted[len(quoted)-1] != '\'' {
			t.Fatalf("luaQuote(%q) is not a quoted literal: %q", value, quoted)
		}
		decoded, ok := unescapeLuaLiteral(quoted[1 : len(quoted)-1])
		if !ok {
			t.Fatalf("luaQuote(%q) escaped in an unknown way: %q", value, quoted)
		}
		if decoded != value {
			t.Fatalf("luaQuote(%q) did not round-trip: %q", value, decoded)
		}
	})
}

func unescapeLuaLiteral(value string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			out.WriteByte(value[i])
			continue
		}
		i++
		if i == len(value) {
			return "", false
		}
		switch value[i] {
		case '\\':
			out.WriteByte('\\')
		case '\'':
			out.WriteByte('\'')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		default:
			return "", false
		}
	}
	return out.String(), true
}

func FuzzSectionValidation(f *testing.F) {
	for _, seed := range []string{"", "lan", "guest2g", "a/b", "a b", "日本語", "a\x00b", strings.Repeat("a", 64), strings.Repeat("a", 65)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if ValidSection(value) && (value == "" || len(value) > 64 || strings.ContainsFunc(value, unicode.IsControl)) {
			t.Fatalf("ValidSection accepted an unsafe value: %q", value)
		}
		if ValidSSID(value) && (len(value) < 1 || len(value) > 32 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl)) {
			t.Fatalf("ValidSSID accepted an unsafe value: %q", value)
		}
	})
}

func FuzzValidOption(f *testing.F) {
	f.Add("firewall", "rule", "proto", "tcp")
	f.Add("system", "system", "hostname", "router")
	f.Add("", "", "", "")
	f.Add("firewall", "rule", "name", "line\nbreak")
	f.Fuzz(func(t *testing.T, pkg, kind, key, value string) {
		if !validOption(pkg, kind, key, value) {
			return
		}
		if allowedOptionsFor(pkg, kind) == nil {
			t.Fatalf("validOption accepted an unknown scope %q.%q", pkg, kind)
		}
		if len(value) > 1024 || strings.ContainsFunc(value, unicode.IsControl) {
			t.Fatalf("validOption accepted an unsafe value: %q", value)
		}
	})
}

func FuzzMergeManagedConfiguration(f *testing.F) {
	f.Add("full:rule0|rule|firewall\nchanges:rule0|rule|firewall|0\nchanges:rule1|rule|firewall|1")
	f.Add("full:a|rule|firewall\nfull:b|rule|firewall\nchanges:a|rule|firewall|1")
	f.Fuzz(func(t *testing.T, spec string) {
		full, changes := decodeConfigurationSpec(spec)
		merged := mergeManagedConfiguration(full, changes)
		for _, change := range changes {
			found := false
			for _, section := range merged {
				if section.Name == change.Name && section.Package == change.Package && section.Type == change.Type {
					found = true
					break
				}
			}
			if change.Absent && found {
				t.Fatalf("absent change %q remained in the merged configuration", change.Name)
			}
			if !change.Absent && !found {
				t.Fatalf("present change %q is missing from the merged configuration", change.Name)
			}
		}
	})
}

func decodeConfigurationSpec(spec string) (Configuration, Configuration) {
	var full, changes Configuration
	for _, line := range strings.Split(spec, "\n") {
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		kind, name, sectionType := parts[0], parts[1], parts[2]
		pkg := "firewall"
		if len(parts) > 3 {
			pkg = parts[3]
		}
		section := Section{Package: pkg, Name: name, Type: sectionType, Options: map[string]string{"name": name}}
		switch kind {
		case "full":
			full = append(full, section)
		case "changes":
			section.Absent = len(parts) > 4 && parts[4] == "1"
			changes = append(changes, section)
		}
	}
	return full, changes
}

func FuzzParseSafetyState(f *testing.F) {
	f.Add(`{"network":[],"firewall":[{"name":"rule","type":"rule","options":{"target":"REJECT"}}]}`)
	f.Add(`{"network":{}}`)
	f.Add(`not json`)
	f.Fuzz(func(t *testing.T, input string) {
		state, err := parseSafetyState([]byte(input))
		if err != nil {
			return
		}
		for packageName, sections := range state {
			for index, section := range sections {
				if section.Options != nil && len(section.Options) == 0 {
					t.Fatalf("%s[%d] kept an empty options map", packageName, index)
				}
				if section.Lists != nil && len(section.Lists) == 0 {
					t.Fatalf("%s[%d] kept an empty lists map", packageName, index)
				}
			}
		}
	})
}

func FuzzParseConfigurationState(f *testing.F) {
	f.Add(`[{"package":"firewall","name":"rule","type":"rule","options":{"target":"REJECT"}}]`, 1)
	f.Add(`[]`, 0)
	f.Add(`{}`, 1)
	f.Fuzz(func(t *testing.T, input string, expected int) {
		configuration, err := parseConfigurationState([]byte(input), expected)
		if err != nil {
			return
		}
		if len(configuration) != expected {
			t.Fatalf("parsed %d sections, want %d", len(configuration), expected)
		}
		for index, section := range configuration {
			if section.Options != nil && len(section.Options) == 0 {
				t.Fatalf("[%d] kept an empty options map", index)
			}
			if section.Lists != nil && len(section.Lists) == 0 {
				t.Fatalf("[%d] kept an empty lists map", index)
			}
		}
	})
}

func FuzzSectionMatches(f *testing.F) {
	f.Add("rule", "rule", "src", "lan")
	f.Add("@rule[0]", "rule", "name", "web")
	f.Add("", "", "", "")
	f.Fuzz(func(t *testing.T, selectorName, actualName, matchKey, matchValue string) {
		selector := Section{Package: "firewall", Type: "rule", Name: selectorName, Match: map[string]string{matchKey: matchValue}}
		actual := Section{Package: "firewall", Type: "rule", Name: actualName, Options: map[string]string{matchKey: matchValue}}
		if sectionMatches(selector, actual) != sectionMatches(selector, actual) {
			t.Fatal("sectionMatches is not deterministic")
		}
	})
}
