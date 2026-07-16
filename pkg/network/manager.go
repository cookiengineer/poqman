package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/cookiengineer/poqman/pkg/storage"
)

const (
	DefaultBridgeName = "poqman0"
	DefaultSubnet     = "10.88.0.0/16"
	DefaultGateway    = "10.88.0.1"
)

type Manager struct {
	paths       *storage.Paths
	BridgeName  string
	Subnet      string
	Gateway     string
	initialized bool
	mu          sync.Mutex
}

type NetworkState struct {
	Bridge      string              `json:"bridge"`
	Subnet      string              `json:"subnet"`
	Gateway     string              `json:"gateway"`
	Allocations map[string]string   `json:"allocations"`
}

func NewManager(paths *storage.Paths) *Manager {
	return &Manager{
		paths:      paths,
		BridgeName: DefaultBridgeName,
		Subnet:     DefaultSubnet,
		Gateway:    DefaultGateway,
	}
}

func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	if err := m.ensureBridge(); err != nil {
		return fmt.Errorf("ensure bridge %s: %w", m.BridgeName, err)
	}

	if err := m.ensureNAT(); err != nil {
		return fmt.Errorf("ensure NAT rules: %w", err)
	}

	if err := m.enableForwarding(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}

	state, err := m.loadState()
	if err == nil {
		m.BridgeName = state.Bridge
		m.Subnet = state.Subnet
		m.Gateway = state.Gateway
	}

	m.initialized = true
	return nil
}

func (m *Manager) ensureBridge() error {
	out, err := exec.Command("ip", "link", "show", m.BridgeName).CombinedOutput()
	if err == nil && strings.Contains(string(out), m.BridgeName) {
		return nil
	}

	cmds := [][]string{
		{"ip", "link", "add", m.BridgeName, "type", "bridge"},
		{"ip", "addr", "add", m.Gateway + "/16", "dev", m.BridgeName},
		{"ip", "link", "set", m.BridgeName, "up"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(cmd, " "), err, string(out))
		}
	}

	return nil
}

func (m *Manager) ensureNAT() error {
	subnet := m.Subnet

	rules := [][]string{
		{"iptables", "-t", "nat", "-C", "POSTROUTING", "-s", subnet, "-j", "MASQUERADE"},
		{"iptables", "-C", "FORWARD", "-i", m.BridgeName, "-j", "ACCEPT"},
		{"iptables", "-C", "FORWARD", "-o", m.BridgeName, "-j", "ACCEPT"},
	}

	addRules := [][]string{
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-s", subnet, "-j", "MASQUERADE"},
		{"iptables", "-A", "FORWARD", "-i", m.BridgeName, "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-o", m.BridgeName, "-j", "ACCEPT"},
	}

	for i, rule := range rules {
		err := exec.Command(rule[0], rule[1:]...).Run()
		if err != nil {
			addCmd := addRules[i]
			out, err := exec.Command(addCmd[0], addCmd[1:]...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s: %w\n%s", strings.Join(addCmd, " "), err, string(out))
			}
		}
	}

	return nil
}

func (m *Manager) enableForwarding() error {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return fmt.Errorf("read ip_forward: %w", err)
	}
	if strings.TrimSpace(string(data)) == "1" {
		return nil
	}
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1\n"), 0o644)
}

func (m *Manager) CreateTap(containerID string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tapName := "tap-" + containerID[:12]

	if _, err := exec.Command("ip", "link", "show", tapName).CombinedOutput(); err == nil {
		return tapName, "", fmt.Errorf("tap device %s already exists", tapName)
	}

	cmds := [][]string{
		{"ip", "tuntap", "add", tapName, "mode", "tap"},
		{"ip", "link", "set", tapName, "master", m.BridgeName},
		{"ip", "link", "set", tapName, "up"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("%s: %w\n%s", strings.Join(cmd, " "), err, string(out))
		}
	}

	state, _ := m.loadState()
	ip, err := allocateIP(state)
	if err != nil {
		m.DestroyTap(tapName)
		return "", "", fmt.Errorf("allocate IP: %w", err)
	}

	state.Allocations[containerID] = ip
	m.saveState(state)

	return tapName, ip, nil
}

func (m *Manager) DestroyTap(tapName string) error {
	out, err := exec.Command("ip", "link", "del", tapName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete tap %s: %w\n%s", tapName, err, string(out))
	}
	return nil
}

func (m *Manager) ReleaseIP(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadState()
	if err != nil {
		return err
	}
	delete(state.Allocations, containerID)
	return m.saveState(state)
}

func (m *Manager) AddPortForward(hostIP string, hostPort int, guestIP string, guestPort int, proto string) error {
	proto = strings.ToLower(proto)
	if proto == "" {
		proto = "tcp"
	}

	rule := []string{
		"iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", proto,
		"-m", proto,
		"--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", guestIP, guestPort),
	}

	if hostIP != "" && hostIP != "0.0.0.0" {
		rule = append(rule[:4], append([]string{"-d", hostIP}, rule[4:]...)...)
	}

	out, err := exec.Command(rule[0], rule[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("add port forward: %w\n%s", err, string(out))
	}

	return nil
}

func (m *Manager) RemovePortForward(hostPort int, proto string) {
	proto = strings.ToLower(proto)
	if proto == "" {
		proto = "tcp"
	}

	exec.Command("iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", proto, "-m", proto, "--dport", fmt.Sprintf("%d", hostPort),
		"-j", "DNAT").Run()
}

func (m *Manager) loadState() (*NetworkState, error) {
	path := m.paths.NetworkStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &NetworkState{
			Bridge:      m.BridgeName,
			Subnet:      m.Subnet,
			Gateway:     m.Gateway,
			Allocations: make(map[string]string),
		}, nil
	}

	var state NetworkState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse network state: %w", err)
	}
	if state.Allocations == nil {
		state.Allocations = make(map[string]string)
	}
	return &state, nil
}

func (m *Manager) saveState(state *NetworkState) error {
	path := m.paths.NetworkStatePath()
	if err := os.MkdirAll(m.paths.Networks, storage.DefaultPerms); err != nil {
		return fmt.Errorf("create network dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal network state: %w", err)
	}
	return os.WriteFile(path, data, storage.FilePerms)
}

func allocateIP(state *NetworkState) (string, error) {
	_, subnet, err := net.ParseCIDR(state.Subnet)
	if err != nil {
		return "", fmt.Errorf("parse subnet %s: %w", state.Subnet, err)
	}

	baseIP := subnet.IP.To4()
	if baseIP == nil {
		return "", fmt.Errorf("IPv4 subnet required, got %s", state.Subnet)
	}

	gatewayIP := net.ParseIP(state.Gateway)
	used := make(map[string]bool)
	used[gatewayIP.String()] = true
	for _, ip := range state.Allocations {
		used[ip] = true
	}

	ip := make(net.IP, len(baseIP))
	copy(ip, baseIP)

	for i := 2; i < 254; i++ {
		ip[3] = byte(i)
		ipStr := ip.String()
		if !used[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet %s", state.Subnet)
}
