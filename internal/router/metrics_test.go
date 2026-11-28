package router

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readTransactionRecords(t *testing.T, path string) []transactionRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	records := []transactionRecord{}
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var record transactionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid transaction record %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func splitLines(value string) []string {
	result := []string{}
	current := ""
	for _, r := range value {
		if r == '\n' {
			result = append(result, current)
			current = ""
			continue
		}
		current += string(r)
	}
	result = append(result, current)
	return result
}

func TestTransactionLogRecordsApplyCost(t *testing.T) {
	before := Configuration{{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "OLD"}}}
	after := Configuration{{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "NEW"}}}
	remote := &configurationMutationRunner{current: before, wanted: after}
	client := newFixtureClient(remote)
	client.AllowNetworkDisruption(true)
	path := filepath.Join(t.TempDir(), "transactions.jsonl")
	client.SetTransactionLog(path)
	if err := client.writeConfiguration(context.Background(), after, nil, after, nil, nil); err != nil {
		t.Fatal(err)
	}
	records := readTransactionRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("recorded %d transactions", len(records))
	}
	record := records[0]
	if record.Result != "committed" {
		t.Fatalf("result = %q", record.Result)
	}
	if record.Changed != 1 || record.Restarts != 1 || len(record.RestartNames) != 1 || record.RestartNames[0] != "samba4" {
		t.Fatalf("unexpected transaction cost: %+v", record)
	}
	if record.Reboots != 0 || record.NetworkCutting {
		t.Fatalf("samba change reported a reboot or network cut: %+v", record)
	}
}

func TestTransactionLogRecordsRebootAndRollback(t *testing.T) {
	before := Configuration{{Package: "network", Name: "guest", Type: "interface", Options: map[string]string{"ipaddr": "192.168.9.1"}}}
	after := Configuration{{Package: "network", Name: "guest", Type: "interface", Options: map[string]string{"ipaddr": "192.168.9.2"}}}
	remote := &configurationMutationRunner{current: before, wanted: after}
	client := newFixtureClient(remote)
	client.AllowNetworkDisruption(true)
	path := filepath.Join(t.TempDir(), "transactions.jsonl")
	client.SetTransactionLog(path)
	if err := client.writeConfiguration(context.Background(), after, nil, after, nil, nil); err != nil {
		t.Fatal(err)
	}
	records := readTransactionRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("recorded %d transactions", len(records))
	}
	if records[0].Reboots != 1 || !records[0].NetworkCutting {
		t.Fatalf("network change did not report its reboot: %+v", records[0])
	}
}

func TestTransactionLogRecordsRollbackOutcome(t *testing.T) {
	before := Configuration{{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "OLD"}}}
	after := Configuration{{Package: "samba4", Name: "samba", Type: "samba", Options: map[string]string{"workgroup": "NEW"}}}
	remote := &configurationMutationRunner{current: before, wanted: after, failApply: true}
	client := newFixtureClient(remote)
	client.AllowNetworkDisruption(true)
	path := filepath.Join(t.TempDir(), "transactions.jsonl")
	client.SetTransactionLog(path)
	if err := client.writeConfiguration(context.Background(), after, nil, after, nil, nil); err == nil {
		t.Fatal("failed apply was accepted")
	}
	records := readTransactionRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("recorded %d transactions", len(records))
	}
	if records[0].Result != "rollback:unknown" && records[0].Result != "rollback:restored" {
		t.Fatalf("rollback result = %q", records[0].Result)
	}
}
