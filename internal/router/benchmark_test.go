package router

import (
	"fmt"
	"testing"
)

func BenchmarkQuote(b *testing.B) {
	value := "a b'c\nd$(echo pwned)"
	for i := 0; i < b.N; i++ {
		_ = quote(value)
	}
}

func BenchmarkLuaQuote(b *testing.B) {
	value := "a'b\\c\nd"
	for i := 0; i < b.N; i++ {
		_ = luaQuote(value)
	}
}

func BenchmarkConfigurationLua(b *testing.B) {
	config := make(Configuration, 0, 64)
	for i := 0; i < 64; i++ {
		config = append(config, Section{
			Package: "firewall", Name: fmt.Sprintf("rule%d", i), Type: "rule",
			Options: map[string]string{"name": fmt.Sprintf("rule%d", i), "target": "REJECT", "enabled": "1", "src": "lan", "dest": "wan"},
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = configurationLua(config, false)
	}
}

func BenchmarkMergeManagedConfiguration(b *testing.B) {
	full := Configuration{}
	changes := Configuration{}
	for i := 0; i < 32; i++ {
		section := Section{Package: "firewall", Name: fmt.Sprintf("rule%d", i), Type: "rule", Options: map[string]string{"name": fmt.Sprintf("rule%d", i)}}
		full = append(full, section)
		changes = append(changes, Section{Package: "firewall", Name: section.Name, Type: "rule", Options: map[string]string{"name": section.Name, "target": "ACCEPT"}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mergeManagedConfiguration(full, changes)
	}
}
