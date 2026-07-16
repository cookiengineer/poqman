package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/container"
	"github.com/cookiengineer/poqman/pkg/image"
)

func TestRegisterInspect(t *testing.T) {
	r := NewRouter()
	RegisterInspect(r)
	if _, ok := r.commands["inspect"]; !ok {
		t.Error("expected 'inspect' command to be registered")
	}
}

func TestContainerInspectResult_Stopped(t *testing.T) {
	now := time.Now()
	c := &container.Container{
		ID:         "stopped-123",
		ImageID:    "sha256:img",
		ImageName:  "busybox:latest",
		Command:    []string{"/bin/sleep", "10"},
		Status:     container.StatusStopped,
		PID:        0,
		ExitCode:   137,
		CreatedAt:  now.Add(-1 * time.Hour),
		StartedAt:  now.Add(-30 * time.Minute),
		FinishedAt: now,
	}

	ci := containerInspectResult(c, nil)
	if ci.ID != "stopped-123" {
		t.Errorf("expected stopped-123, got %s", ci.ID)
	}
	if ci.Status != "stopped" {
		t.Errorf("expected stopped, got %s", ci.Status)
	}
	if ci.PID != 0 {
		t.Errorf("expected PID 0 for stopped, got %d", ci.PID)
	}
	if ci.ExitCode != 137 {
		t.Errorf("expected exit 137, got %d", ci.ExitCode)
	}
	if ci.FinishedAt == "" {
		t.Error("expected non-empty finishedAt for stopped container")
	}
}

func TestContainerInspectResult_Failed(t *testing.T) {
	c := &container.Container{
		ID:        "failed-1",
		ImageID:   "sha256:img",
		ImageName: "broken:latest",
		Status:    container.StatusFailed,
		ExitCode:  1,
		CreatedAt: time.Now(),
	}

	ci := containerInspectResult(c, nil)
	if ci.Status != "failed" {
		t.Errorf("expected failed, got %s", ci.Status)
	}
	if ci.PID != 0 {
		t.Errorf("expected PID 0 for failed, got %d", ci.PID)
	}
}

func TestContainerInspectResult_Minimal(t *testing.T) {
	c := &container.Container{
		ID:        "minimal-1",
		ImageID:   "sha256:img",
		ImageName: "scratch",
		Status:    container.StatusCreated,
		CreatedAt: time.Now(),
	}

	ci := containerInspectResult(c, nil)
	if ci.ID != "minimal-1" {
		t.Errorf("expected minimal-1, got %s", ci.ID)
	}
	if ci.Command != nil {
		t.Errorf("expected nil command, got %v", ci.Command)
	}
	if ci.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", ci.ExitCode)
	}
}

func TestImageInspectResult_FullConfig(t *testing.T) {
	img := &image.Image{
		ID:       "sha256:full",
		RepoTags: []string{"full:latest", "full:v1"},
		Arch:     "arm64",
		Config: image.ImageConfig{
			User:         "nobody",
			Env:          []string{"PATH=/usr/bin", "HOME=/root"},
			Cmd:          []string{"nginx", "-g"},
			Entrypoint:   []string{"/docker-entrypoint.sh"},
			Workdir:      "/app",
			ExposedPorts: map[string]struct{}{"80/tcp": {}, "443/tcp": {}},
			Volumes:      map[string]struct{}{"/data": {}},
			Labels:       map[string]string{"org.opencontainers.image.version": "1.0"},
			StopSignal:   "SIGQUIT",
			Shell:        []string{"/bin/ash", "-c"},
		},
		Layers: []image.Layer{
			{Digest: "sha256:l1", Size: 1000, MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
			{Digest: "sha256:l2", Size: 2000, MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
			{Digest: "sha256:l3", Size: 3000, MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
		},
		KernelRef: "debian:6.1.0-25",
		Created:   time.Now(),
		Size:      6000,
	}

	ii := imageInspectResult(img, nil)
	if ii.Arch != "arm64" {
		t.Errorf("expected arm64, got %s", ii.Arch)
	}
	if len(ii.RepoTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(ii.RepoTags))
	}
	if len(ii.Layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(ii.Layers))
	}
	if ii.Config.User != "nobody" {
		t.Errorf("expected user nobody, got %s", ii.Config.User)
	}
	if ii.Config.Workdir != "/app" {
		t.Errorf("expected workdir /app, got %s", ii.Config.Workdir)
	}
	if len(ii.Config.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(ii.Config.Env))
	}
}

func TestImageInspectResult_EmptyImage(t *testing.T) {
	img := &image.Image{
		ID:      "sha256:empty",
		Arch:    "amd64",
		Created: time.Time{},
		Size:    0,
	}

	ii := imageInspectResult(img, nil)
	if ii.ID != "sha256:empty" {
		t.Errorf("expected sha256:empty, got %s", ii.ID)
	}
	if ii.Created != "" {
		t.Errorf("expected empty created for zero time, got %s", ii.Created)
	}
	if ii.Arch != "amd64" {
		t.Errorf("expected amd64, got %s", ii.Arch)
	}
	if len(ii.RepoTags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(ii.RepoTags))
	}
}

func TestContainerInspect_MultiplePorts(t *testing.T) {
	c := &container.Container{
		ID:     "multi-port",
		Status: container.StatusRunning,
		Ports: []container.PortMapping{
			{HostPort: 8080, GuestPort: 80, Protocol: "tcp"},
			{HostPort: 8443, GuestPort: 443, Protocol: "tcp"},
			{HostPort: 53, GuestPort: 53, Protocol: "udp"},
		},
		CreatedAt: time.Now(),
	}

	ci := containerInspectResult(c, nil)
	if len(ci.Ports) != 3 {
		t.Errorf("expected 3 ports, got %d", len(ci.Ports))
	}
	if ci.Ports[2].Protocol != "udp" {
		t.Errorf("expected udp on 3rd port, got %s", ci.Ports[2].Protocol)
	}
}

func TestContainerInspect_JSONRoundTrip(t *testing.T) {
	c := &container.Container{
		ID:        "round-trip",
		ImageID:   "sha256:img",
		ImageName: "test:latest",
		Command:   []string{"/bin/sh", "-c", "echo hello world"},
		Status:    container.StatusRunning,
		PID:       42,
		IP:        "10.88.0.5",
		Ports: []container.PortMapping{
			{HostPort: 8080, GuestPort: 80, Protocol: "tcp"},
		},
		Volumes: []container.VolumeMount{
			{Source: "/host/a", Target: "/guest/a"},
			{Source: "/host/b", Target: "/guest/b", ReadOnly: true},
		},
		CreatedAt:  time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		StartedAt:  time.Date(2026, 7, 16, 12, 0, 5, 0, time.UTC),
		FinishedAt: time.Time{},
		ExitCode:   0,
	}

	ci := containerInspectResult(c, nil)
	data, err := json.Marshal(ci)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ContainerInspect
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "round-trip" {
		t.Errorf("round-trip: expected round-trip, got %s", decoded.ID)
	}
	if decoded.PID != 42 {
		t.Errorf("round-trip: expected PID 42, got %d", decoded.PID)
	}
	if decoded.Status != "running" {
		t.Errorf("round-trip: expected running, got %s", decoded.Status)
	}
	if len(decoded.Volumes) != 2 {
		t.Errorf("round-trip: expected 2 volumes, got %d", len(decoded.Volumes))
	}
	if decoded.ExitCode != 0 {
		t.Errorf("round-trip: expected exit 0, got %d", decoded.ExitCode)
	}
}

func TestImageInspect_JSONRoundTrip(t *testing.T) {
	img := &image.Image{
		ID:       "sha256:round",
		RepoTags: []string{"round:tag"},
		Arch:     "amd64",
		Config: image.ImageConfig{
			Env:          []string{"A=1"},
			Cmd:          []string{"sh"},
			ExposedPorts: map[string]struct{}{"80/tcp": {}},
			Volumes:      map[string]struct{}{"/data": {}},
		},
		Layers:    []image.Layer{{Digest: "sha256:l1", Size: 100, MediaType: "test"}},
		KernelRef: "oci://kernel:1",
		Created:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Size:      100,
	}

	ii := imageInspectResult(img, nil)
	data, _ := json.Marshal(ii)
	var decoded ImageInspect
	json.Unmarshal(data, &decoded)

	if decoded.ID != "sha256:round" {
		t.Errorf("round-trip: expected sha256:round, got %s", decoded.ID)
	}
	if decoded.KernelRef != "oci://kernel:1" {
		t.Errorf("round-trip: expected kernel ref preserved, got %s", decoded.KernelRef)
	}
	if len(decoded.Layers) != 1 {
		t.Errorf("round-trip: expected 1 layer, got %d", len(decoded.Layers))
	}
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2026, 7, 16, 14, 30, 0, 0, time.UTC)
	got := formatTime(ts)
	if !strings.Contains(got, "2026-07-16") {
		t.Errorf("expected date in output, got %s", got)
	}
	if formatTime(time.Time{}) != "" {
		t.Error("expected empty string for zero time")
	}

	ts2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got2 := formatTime(ts2)
	if !strings.Contains(got2, "T00:00:00Z") {
		t.Errorf("expected midnight UTC, got %s", got2)
	}
}

func TestPrintJSON_StringOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := map[string]string{"key": "value"}
	err := printJSON(v)
	if err != nil {
		t.Errorf("printJSON should not error: %v", err)
	}

	v2 := []int{1, 2, 3}
	err = printJSON(v2)
	if err != nil {
		t.Errorf("printJSON array should not error: %v", err)
	}

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if buf.Len() == 0 {
		t.Error("expected output from printJSON")
	}
}
