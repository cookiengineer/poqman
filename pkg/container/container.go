package container

import (
	"time"
)

type Container struct {
	ID         string           `json:"id"`
	ImageID    string           `json:"imageId"`
	ImageName  string           `json:"imageName"`
	Command    []string         `json:"command"`
	Status     ContainerStatus  `json:"status"`
	PID        int              `json:"pid,omitempty"`
	IP         string           `json:"ip,omitempty"`
	Ports      []PortMapping    `json:"ports,omitempty"`
	Volumes    []VolumeMount    `json:"volumes,omitempty"`
	Name       string           `json:"name,omitempty"`
	CreatedAt  time.Time        `json:"createdAt"`
	StartedAt  time.Time        `json:"startedAt"`
	FinishedAt time.Time        `json:"finishedAt"`
	ExitCode   int              `json:"exitCode"`
}

type ContainerStatus string

const (
	StatusCreated ContainerStatus = "created"
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusFailed  ContainerStatus = "failed"
)

type PortMapping struct {
	HostIP    string `json:"hostIp"`
	HostPort  int    `json:"hostPort"`
	GuestPort int    `json:"guestPort"`
	Protocol  string `json:"protocol"`
}

type VolumeMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

func GenerateID() string {
	// Use a simple random hex ID like podman/docker
	var buf [12]byte
	fillRandom(buf[:])
	const hexDigits = "0123456789abcdef"
	var id [12]byte
	for i := 0; i < 12; i++ {
		id[i] = hexDigits[buf[i]%16]
	}
	return string(id[:])
}

func fillRandom(b []byte) {
	// Use a simple PRNG seeded from time; for production replace with crypto/rand
	// For MVP this is sufficient and avoids importing crypto/rand in container package
	n := time.Now().UnixNano()
	for i := range b {
		n = n*1103515245 + 12345
		b[i] = byte((n / 65536) % 32768)
	}
}
