package router

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func verifyWiredManagement(ctx context.Context, runner Runner, host string, port int) error {
	output, err := runner.Run(ctx, preflight+"uci -q get network.lan.ipaddr\nuci -q get network.lan.netmask\n")
	if err != nil {
		return errors.New("Cannot inspect the private LAN management address before a disruptive change.")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "udp4", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return errors.New("Cannot determine the management route before a disruptive change.")
	}
	defer connection.Close()
	local := connection.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap()
	remote := connection.RemoteAddr().(*net.UDPAddr).AddrPort().Addr().Unmap()
	values := strings.Fields(string(output))
	if len(values) != 2 {
		return errors.New("The router returned an invalid private LAN management address.")
	}
	bits, err := netip.ParseAddr(values[1])
	if err != nil || !bits.Is4() {
		return errors.New("The router returned an invalid private LAN netmask.")
	}
	mask := bits.As4()
	ones, size := net.IPMask(mask[:]).Size()
	gateway, err := netip.ParseAddr(values[0])
	if err != nil || !gateway.Is4() || !gateway.IsPrivate() || ones == 0 || size != 32 || remote != gateway || !netip.PrefixFrom(gateway, ones).Contains(local) {
		return errors.New("Disruptive changes require a direct wired connection to the router's private LAN address.")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return errors.New("Cannot inspect the local management interfaces.")
	}
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil || prefix.Addr().Unmap() != local {
				continue
			}
			if iface.Flags&net.FlagUp == 0 || iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 || len(iface.HardwareAddr) != 6 {
				return errors.New("The management route is not an active Ethernet interface.")
			}
			return verifyEthernetInterface(ctx, iface.Name)
		}
	}
	return errors.New("Cannot identify the local management interface.")
}

func verifyEthernetInterface(ctx context.Context, name string) error {
	switch runtime.GOOS {
	case "darwin":
		output, err := exec.CommandContext(ctx, "/sbin/ifconfig", name).Output()
		if err == nil && activeEthernetLink(string(output)) {
			return nil
		}
	case "linux":
		base := filepath.Join("/sys/class/net", name)
		for _, wireless := range []string{"wireless", "phy80211"} {
			if _, err := os.Stat(filepath.Join(base, wireless)); !errors.Is(err, os.ErrNotExist) {
				return errors.New("Disruptive changes cannot use a Wi-Fi management route.")
			}
		}
		if _, err := os.Stat(filepath.Join(base, "device")); err != nil {
			return errors.New("Disruptive changes require a physical Ethernet management interface.")
		}
		carrier, err := os.ReadFile(filepath.Join(base, "carrier"))
		if err == nil && strings.TrimSpace(string(carrier)) == "1" {
			return nil
		}
	}
	return errors.New("Cannot prove an active wired Ethernet management path. No configuration was changed.")
}

func activeEthernetLink(output string) bool {
	return strings.Contains(output, "status: active") && regexp.MustCompile(`(?m)media:.*(?:baseT|base-F|baseX)`).MatchString(output)
}
