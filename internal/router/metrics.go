package router

import (
	"encoding/json"
	"os"
	"time"
)

type transactionRecord struct {
	At             string   `json:"at"`
	Result         string   `json:"result"`
	TotalMS        int64    `json:"total_ms"`
	BackupMS       int64    `json:"backup_ms"`
	ApplyMS        int64    `json:"apply_ms"`
	VerifyMS       int64    `json:"verify_ms"`
	ConfirmMS      int64    `json:"confirm_ms"`
	Changed        int      `json:"changed_sections"`
	Reboots        int      `json:"reboots"`
	Restarts       int      `json:"restarts"`
	RestartNames   []string `json:"restart_names,omitempty"`
	NetworkCutting bool     `json:"network_disruptive"`
}

// SetTransactionLog appends one JSON line per configuration transaction to
// path. It is a diagnostic aid for measuring apply cost on hardware and never
// changes the transaction outcome.
func (c *Client) SetTransactionLog(path string) {
	c.transactionLog = path
}

func (c *Client) recordTransaction(record transactionRecord) {
	if c.transactionLog == "" {
		return
	}
	if record.RestartNames == nil {
		record.RestartNames = []string{}
	}
	record.At = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	file, err := os.OpenFile(c.transactionLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

func transactionCost(plan reloadPlan, elapsed [4]time.Duration, changed int, result string, started time.Time) transactionRecord {
	record := transactionRecord{
		Result:         result,
		TotalMS:        time.Since(started).Milliseconds(),
		BackupMS:       elapsed[0].Milliseconds(),
		ApplyMS:        elapsed[1].Milliseconds(),
		VerifyMS:       elapsed[2].Milliseconds(),
		ConfirmMS:      elapsed[3].Milliseconds(),
		Changed:        changed,
		NetworkCutting: networkDisruptionRequired(plan),
	}
	if plan.Reboot {
		record.Reboots = 1
	}
	if plan.Wireless {
		record.Restarts++
	}
	if plan.DNS || plan.DHCP {
		record.Restarts++
	}
	if plan.Firewall {
		record.Restarts++
	}
	if plan.Reboot {
		record.Restarts++
	}
	for _, service := range plan.Services {
		record.RestartNames = append(record.RestartNames, service.Name)
		record.Restarts++
	}
	return record
}
