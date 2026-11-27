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
