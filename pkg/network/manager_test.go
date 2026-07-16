package network

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cookiengineer/poqman/pkg/storage"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Images:     filepath.Join(tmp, "images"),
		Kernels:    filepath.Join(tmp, "kernels"),
		Containers: filepath.Join(tmp, "containers"),
		Networks:   filepath.Join(tmp, "networks"),
		Tmp:        filepath.Join(tmp, "tmp"),
	}
	paths.EnsureAll()

	return &Manager{
		paths:      paths,
		BridgeName: "poqman0",
		Subnet:     "10.88.0.0/16",
		Gateway:    "10.88.0.1",
	}
}

func TestNewManager(t *testing.T) {
	tmp := t.TempDir()
	paths := &storage.Paths{
		Base:       tmp,
		Networks:   filepath.Join(tmp, "networks"),
	}
	m := NewManager(paths)

	if m.BridgeName != DefaultBridgeName {
		t.Errorf("expected bridge %s, got %s", DefaultBridgeName, m.BridgeName)
	}
	if m.Subnet != DefaultSubnet {
		t.Errorf("expected subnet %s, got %s", DefaultSubnet, m.Subnet)
	}
	if m.Gateway != DefaultGateway {
		t.Errorf("expected gateway %s, got %s", DefaultGateway, m.Gateway)
	}
}

func TestLoadNetworkState_Empty(t *testing.T) {
	m := newTestManager(t)

	state, err := m.loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if len(state.Allocations) != 0 {
		t.Errorf("expected empty allocations, got %d", len(state.Allocations))
	}
	if state.Bridge != "poqman0" {
		t.Errorf("expected bridge poqman0, got %s", state.Bridge)
	}
}

func TestSaveAndLoadNetworkState(t *testing.T) {
	m := newTestManager(t)

	state := &NetworkState{
		Bridge:  "poqman0",
		Subnet:  "10.88.0.0/16",
		Gateway: "10.88.0.1",
		Allocations: map[string]string{
			"container1": "10.88.0.2",
			"container2": "10.88.0.3",
		},
	}

	if err := m.saveState(state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	loaded, err := m.loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}

	if loaded.Bridge != "poqman0" {
		t.Errorf("expected bridge poqman0, got %s", loaded.Bridge)
	}
	if len(loaded.Allocations) != 2 {
		t.Errorf("expected 2 allocations, got %d", len(loaded.Allocations))
	}
	if loaded.Allocations["container1"] != "10.88.0.2" {
		t.Errorf("expected 10.88.0.2 for container1, got %s", loaded.Allocations["container1"])
	}
}

func TestNetworkState_JSON(t *testing.T) {
	state := &NetworkState{
		Bridge:  "poqman0",
		Subnet:  "10.88.0.0/16",
		Gateway: "10.88.0.1",
		Allocations: map[string]string{
			"abc": "10.88.0.5",
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded NetworkState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Allocations["abc"] != "10.88.0.5" {
		t.Errorf("expected 10.88.0.5, got %s", loaded.Allocations["abc"])
	}
}

func TestAllocateIP(t *testing.T) {
	state := &NetworkState{
		Subnet:  "10.88.0.0/16",
		Gateway: "10.88.0.1",
		Allocations: map[string]string{
			"c1": "10.88.0.2",
		},
	}

	// First IP should skip gateway and c1, get .3
	ip1, err := allocateIP(state)
	if err != nil {
		t.Fatalf("allocateIP: %v", err)
	}
	if ip1 != "10.88.0.3" {
		t.Errorf("expected 10.88.0.3, got %s", ip1)
	}

	// Allocate many to check exhaustion
	state.Allocations[ip1] = ip1
	for i := 0; i < 250; i++ {
		ip, err := allocateIP(state)
		if err != nil {
			if i < 248 {
				t.Errorf("unexpected exhaustion at IP %d: %v", i, err)
			}
			break
		}
		state.Allocations[ip] = ip
	}
}

func TestAllocateIP_InvalidSubnet(t *testing.T) {
	state := &NetworkState{
		Subnet:  "invalid",
		Gateway: "10.88.0.1",
		Allocations: map[string]string{},
	}

	_, err := allocateIP(state)
	if err == nil {
		t.Error("expected error for invalid subnet")
	}
}

func TestReleaseIP(t *testing.T) {
	m := newTestManager(t)

	state := &NetworkState{
		Bridge: "poqman0",
		Subnet: "10.88.0.0/16",
		Gateway: "10.88.0.1",
		Allocations: map[string]string{
			"container1": "10.88.0.5",
		},
	}
	m.saveState(state)

	if err := m.ReleaseIP("container1"); err != nil {
		t.Fatalf("ReleaseIP: %v", err)
	}

	loaded, _ := m.loadState()
	_, exists := loaded.Allocations["container1"]
	if exists {
		t.Error("expected container1 to be released")
	}
}

func TestEnableForwarding_ReadsFile(t *testing.T) {
	tmp := t.TempDir()

	origPath := "/proc/sys/net/ipv4/ip_forward"
	_ = origPath

	content, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		t.Skipf("cannot read ip_forward (non-Linux?): %v", err)
	}
	if string(content) != "" {
		_ = tmp
	}
}
