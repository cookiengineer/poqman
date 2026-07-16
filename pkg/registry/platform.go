package registry

import (
	"fmt"
	"runtime"
	"strings"
)

type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

func HostPlatform() Platform {
	return Platform{
		Architecture: hostArchitecture(),
		OS:           "linux",
	}
}

func ParsePlatform(raw string) (Platform, error) {
	if raw == "" {
		return HostPlatform(), nil
	}

	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return Platform{}, fmt.Errorf("invalid platform format %q, expected os/arch", raw)
	}

	archParts := strings.SplitN(parts[1], "/", 2)
	variant := ""
	if len(archParts) > 1 {
		variant = archParts[1]
	}

	return Platform{
		OS:           parts[0],
		Architecture: archParts[0],
		Variant:      variant,
	}, nil
}

func (p Platform) String() string {
	if p.Variant != "" {
		return fmt.Sprintf("%s/%s/%s", p.OS, p.Architecture, p.Variant)
	}
	return fmt.Sprintf("%s/%s", p.OS, p.Architecture)
}

func (p Platform) Match(other Platform) bool {
	if p.OS != "" && other.OS != "" && p.OS != other.OS {
		return false
	}
	if p.Architecture != "" && other.Architecture != "" && p.Architecture != other.Architecture {
		return false
	}
	if p.Variant != "" && other.Variant != "" && p.Variant != other.Variant {
		return false
	}
	return true
}

func MatchManifest(entries []ManifestListEntry, target Platform) (*ManifestListEntry, bool) {
	for _, entry := range entries {
		if entry.Platform == nil {
			continue
		}
		if target.Match(*entry.Platform) {
			return &entry, true
		}
	}

	// fallback: try to match just arch+os, ignoring variant
	for _, entry := range entries {
		if entry.Platform == nil {
			continue
		}
		if entry.Platform.Architecture == target.Architecture &&
			entry.Platform.OS == target.OS {
			return &entry, true
		}
	}

	return nil, false
}

func hostArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "arm":
		return "arm"
	case "386":
		return "386"
	case "riscv64":
		return "riscv64"
	case "ppc64le":
		return "ppc64le"
	case "s390x":
		return "s390x"
	default:
		return runtime.GOARCH
	}
}
