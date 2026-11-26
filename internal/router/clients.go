package router

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// Lease is one DHCP client entry reported by the router.
type Lease struct {
	MAC          string
	IP           string
	Hostname     string
	ClientID     string
	LeaseSeconds int64
}

// DHCPLeases reads the dnsmasq lease table. The table is runtime state, so a
// missing file means no clients rather than an error.
func (c *Client) DHCPLeases(ctx context.Context) ([]Lease, error) {
	output, err := c.runner.Run(ctx, "cat /tmp/dhcp.leases 2>/dev/null || true")
	if err != nil {
		return nil, operationError(err)
	}
	return parseLeases(string(output))
}

func parseLeases(data string) ([]Lease, error) {
	leases := []Lease{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 4 {
			return nil, errors.New("The router returned an invalid DHCP lease table.")
		}
		seconds, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, errors.New("The router returned an invalid DHCP lease table.")
		}
		lease := Lease{MAC: fields[1], IP: fields[2], Hostname: fields[3], LeaseSeconds: seconds}
		if len(fields) > 4 && fields[4] != "*" {
			lease.ClientID = fields[4]
		}
		leases = append(leases, lease)
	}
	return leases, nil
}
