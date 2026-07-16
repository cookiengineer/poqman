package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type Kernel struct {
	ID         string    `json:"id"`
	Distro     string    `json:"distro"`
	Version    string    `json:"version"`
	Arch       string    `json:"arch"`
	PackageURL string    `json:"packageUrl"`
	Created    time.Time `json:"created"`
}

type KernelIndex struct {
	Kernels map[string]string `json:"kernels"`
}

func NewKernelIndex() *KernelIndex {
	return &KernelIndex{Kernels: make(map[string]string)}
}

func (idx *KernelIndex) Add(name string, id string) {
	idx.Kernels[name] = id
}

func (idx *KernelIndex) Remove(name string) {
	delete(idx.Kernels, name)
}

func (idx *KernelIndex) Lookup(name string) (string, bool) {
	id, ok := idx.Kernels[name]
	return id, ok
}

func GenerateKernelID(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(hash[:]))
}

type ResolveRequest struct {
	Distro  string
	Version string
	Arch    string
}

func ParseKernelRef(raw string) (*ResolveRequest, error) {
	if raw == "" {
		return nil, fmt.Errorf("kernel reference must not be empty")
	}
	req := &ResolveRequest{}

	idx := indexOf(raw, ":")
	if idx < 0 {
		return nil, fmt.Errorf("invalid kernel reference %q, expected distro:version", raw)
	}

	req.Distro = raw[:idx]
	remainder := raw[idx+1:]

	lastIdx := indexOf(remainder, ":")
	if lastIdx >= 0 {
		potentialArch := remainder[lastIdx+1:]
		if isKnownArch(potentialArch) {
			req.Version = remainder[:lastIdx]
			req.Arch = potentialArch
			return req, nil
		}
		req.Version = remainder
	} else {
		req.Version = remainder
	}

	return req, nil
}

func isKnownArch(arch string) bool {
	switch arch {
	case "amd64", "arm64", "arm", "armhf", "i386", "riscv64", "ppc64le", "s390x", "x86_64", "aarch64":
		return true
	}
	return false
}

func (r *ResolveRequest) String() string {
	if r.Arch != "" {
		return fmt.Sprintf("%s:%s:%s", r.Distro, r.Version, r.Arch)
	}
	return fmt.Sprintf("%s:%s", r.Distro, r.Version)
}

func splitN(s, sep string, n int) []string {
	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			return result
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	if s != "" {
		result = append(result, s)
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
