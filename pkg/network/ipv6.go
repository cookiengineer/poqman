package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

const (
	DefaultIPv6Subnet  = "fd00:dead:beef::/64"
	DefaultIPv6Gateway = "fd00:dead:beef::1"
)

func SetupIPv6(bridgeName, subnet, gateway string) error {
	out, err := exec.Command("ip", "-6", "addr", "show", "dev", bridgeName).CombinedOutput()
	if err == nil && strings.Contains(string(out), gateway) {
		return nil
	}

	cmds := [][]string{
		{"ip", "-6", "addr", "add", gateway + "/64", "dev", bridgeName},
		{"sysctl", "-w", "net.ipv6.conf.all.forwarding=1"},
		{"sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.accept_ra=2", bridgeName)},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(cmd, " "), err, string(out))
		}
	}

	return nil
}

func AllocateIPv6(subnet string, used map[string]bool) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", fmt.Errorf("parse IPv6 subnet: %w", err)
	}

	baseIP := ipNet.IP.To16()
	if baseIP == nil {
		return "", fmt.Errorf("invalid IPv6 subnet")
	}

	ip := make(net.IP, len(baseIP))
	copy(ip, baseIP)

	for i := 0; i < 65535; i++ {
		ip[14] = byte(i >> 8)
		ip[15] = byte(i & 0xff)
		if ip[15] == 0 || ip[15] == 1 {
			continue
		}
		ipStr := ip.String()
		if !used[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IPv6 addresses in subnet %s", subnet)
}

func EnableDHCP(bridgeName string) error {
	out, err := exec.Command("which", "dnsmasq").CombinedOutput()
	if err != nil {
		return fmt.Errorf("dnsmasq not found: %s", string(out))
	}

	return nil
}

func StartDHCPServer(bridgeName, subnet, gateway, dhcpRange string) error {
	cmd := exec.Command("dnsmasq",
		"--interface="+bridgeName,
		"--bind-interfaces",
		"--dhcp-range="+dhcpRange,
		"--dhcp-option=option:router,"+gateway,
		"--dhcp-option=option:dns-server,1.1.1.1",
		"--no-daemon",
		"--log-queries",
	)
	return cmd.Start()
}
