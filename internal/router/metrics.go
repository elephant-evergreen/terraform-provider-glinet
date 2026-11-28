package router

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type transactionRecord struct {
	At             string   `json:"at"`
	Result         string   `json:"result"`
	TotalMS        int64    `json:"total_ms"`
	BackupMS       int64    `json:"backup_ms"`
	ApplyMS        int64    `json:"apply_ms"`
	ReconnectMS    int64    `json:"reconnect_ms"`
	VerifyMS       int64    `json:"verify_ms"`
	ConfirmMS      int64    `json:"confirm_ms"`
	Changed        int      `json:"changed_sections"`
	UCICommits     int      `json:"uci_commits"`
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

type transactionPhases struct {
	Backup    time.Duration
	Apply     time.Duration
	Reconnect time.Duration
	Verify    time.Duration
	Confirm   time.Duration
}

func transactionCost(plan reloadPlan, phases transactionPhases, config Configuration, result string, started time.Time) transactionRecord {
	record := transactionRecord{
		Result:         result,
		TotalMS:        time.Since(started).Milliseconds(),
		BackupMS:       phases.Backup.Milliseconds(),
		ApplyMS:        phases.Apply.Milliseconds(),
		ReconnectMS:    phases.Reconnect.Milliseconds(),
		VerifyMS:       phases.Verify.Milliseconds(),
		ConfirmMS:      phases.Confirm.Milliseconds(),
		Changed:        len(config),
		UCICommits:     len(strings.Fields(configurationCommitPackages(config, plan))),
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
