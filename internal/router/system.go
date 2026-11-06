package router

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type SystemInfo struct {
	Model    string
	Firmware string
	Mode     string
}

func (c *Client) System(ctx context.Context) (SystemInfo, error) {
	boardOutput, err := c.runner.Run(ctx, "ubus call system board")
	if err != nil {
		return SystemInfo{}, operationError(err)
	}
	var board struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(boardOutput, &board) != nil {
		return SystemInfo{}, errors.New("The router returned an invalid board description.")
	}
	firmware, err := c.runner.Run(ctx, "cat /etc/glversion")
	if err != nil {
		return SystemInfo{}, operationError(err)
	}
	mode, err := c.runner.Run(ctx, "uci -q get glconfig.general.mode")
	if err != nil {
		return SystemInfo{}, operationError(err)
	}
	return SystemInfo{Model: board.Model, Firmware: strings.TrimSpace(string(firmware)), Mode: strings.TrimSpace(string(mode))}, nil
}
