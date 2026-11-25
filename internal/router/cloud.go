package router

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const cloudStateScript = `printf '%s\n' '@@config@@'
ubus call gl-session call '{"module":"cloud","func":"get_config"}' 2>/dev/null || true
printf '%s\n' '@@running@@'
pidof gl-cloud 2>/dev/null || true
printf '%s\n' '@@service@@'
ubus call service list '{"name":"gl-cloud"}' 2>/dev/null || true
`

type CloudState struct {
	Enabled    bool
	Running    bool
	Registered bool
}

type ServiceChange struct {
	Cloud    *bool
	Snapshot bool
}

func CloudMarker() Section {
	return Section{Package: "gl-cloud", Type: "cloud", Name: "@cloud[0]", Options: map[string]string{ManagedOption: "1"}}
}

func (c *Client) Cloud(ctx context.Context) (CloudState, error) {
	output, err := c.runner.Run(ctx, preflight+cloudStateScript)
	if err != nil {
		return CloudState{}, operationError(err)
	}
	text := string(output)
	configText := between(text, "@@config@@", "@@running@@")
	runningText := between(text, "@@running@@", "@@service@@")
	serviceText := after(text, "@@service@@")

	var response struct {
		Result *struct {
			CloudEnable json.RawMessage `json:"cloud_enable"`
		} `json:"result"`
	}
	if json.Unmarshal([]byte(configText), &response) != nil || response.Result == nil {
		return CloudState{}, errors.New("The router did not return a GoodCloud configuration.")
	}
	enabled, err := cloudEnabled(response.Result.CloudEnable)
	if err != nil {
		return CloudState{}, err
	}
	var services map[string]json.RawMessage
	registered := json.Unmarshal([]byte(serviceText), &services) == nil
	if registered {
		_, registered = services["gl-cloud"]
	}
	return CloudState{
		Enabled:    enabled,
		Running:    strings.TrimSpace(runningText) != "",
		Registered: registered,
	}, nil
}

func cloudEnabled(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	var value bool
	if json.Unmarshal(raw, &value) == nil {
		return value, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		parsed, err := strconv.ParseBool(text)
		if err == nil {
			return parsed, nil
		}
	}
	return false, errors.New("The router returned an invalid GoodCloud enable value.")
}

func (c *Client) ApplyCloud(ctx context.Context, enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.readSafetyState(ctx)
	if err != nil {
		return err
	}
	if !cloudSectionPresent(state) {
		return errors.New("The router does not expose the supported GoodCloud singleton (`gl-cloud.@cloud[0]`).")
	}
	marker := CloudMarker()
	full := configurationFromManagedState(state)
	full = mergeManagedConfiguration(full, Configuration{marker})
	return c.writeConfiguration(ctx, Configuration{marker}, nil, full, &ServiceChange{Cloud: &enabled, Snapshot: true})
}

func (c *Client) ForgetCloud(ctx context.Context) error {
	marker := CloudMarker()
	_, found, err := c.ReadSection(ctx, marker)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state, err := c.readSafetyState(ctx)
	if err != nil {
		return err
	}
	marker.Options = map[string]string{ManagedOption: "0"}
	full := configurationFromManagedState(state)
	full = mergeManagedConfiguration(full, Configuration{marker})
	return c.writeConfiguration(ctx, Configuration{marker}, nil, full, &ServiceChange{Snapshot: true})
}

func cloudSectionPresent(state uciState) bool {
	for _, section := range state["gl-cloud"] {
		if section.Type == "cloud" {
			return true
		}
	}
	return false
}
