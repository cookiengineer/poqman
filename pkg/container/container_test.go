package container

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if len(id1) != 12 {
		t.Errorf("expected ID length 12, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("expected different IDs for separate calls")
	}

	for _, ch := range id1 {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("ID contains invalid character: %c", ch)
		}
	}
}

func TestContainerStatus_Constants(t *testing.T) {
	if StatusCreated != "created" {
		t.Errorf("StatusCreated = %q, want %q", StatusCreated, "created")
	}
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q, want %q", StatusRunning, "running")
	}
	if StatusStopped != "stopped" {
		t.Errorf("StatusStopped = %q, want %q", StatusStopped, "stopped")
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want %q", StatusFailed, "failed")
	}
}

func TestContainer_Defaults(t *testing.T) {
	c := Container{
		ID:        "abc123def456",
		ImageID:   "sha256:img",
		ImageName: "alpine:latest",
		Status:    StatusCreated,
		CreatedAt: time.Now(),
	}

	if c.Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %s", c.Status)
	}
	if c.ID != "abc123def456" {
		t.Errorf("expected abc123def456, got %s", c.ID)
	}
	if c.PID != 0 {
		t.Errorf("expected PID 0, got %d", c.PID)
	}
}

func TestPortMapping_Defaults(t *testing.T) {
	pm := PortMapping{
		HostPort:  8080,
		GuestPort: 80,
		Protocol:  "tcp",
	}
	if pm.HostPort != 8080 {
		t.Errorf("expected HostPort 8080, got %d", pm.HostPort)
	}
	if pm.GuestPort != 80 {
		t.Errorf("expected GuestPort 80, got %d", pm.GuestPort)
	}
	if pm.Protocol != "tcp" {
		t.Errorf("expected Protocol tcp, got %s", pm.Protocol)
	}
}

func TestVolumeMount_Defaults(t *testing.T) {
	vm := VolumeMount{
		Source:   "/host/data",
		Target:   "/container/data",
		ReadOnly: true,
	}
	if vm.Source != "/host/data" {
		t.Errorf("expected Source /host/data, got %s", vm.Source)
	}
	if vm.Target != "/container/data" {
		t.Errorf("expected Target /container/data, got %s", vm.Target)
	}
	if !vm.ReadOnly {
		t.Error("expected ReadOnly true")
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}
