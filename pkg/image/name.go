package image

import (
	"fmt"
	"strings"
)

const (
	DefaultRegistry   = "docker.io"
	DefaultNamespace  = "library"
	DefaultTag        = "latest"
)

type ImageRef struct {
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

func ParseImageRef(raw string) (ImageRef, error) {
	if raw == "" {
		return ImageRef{}, fmt.Errorf("image reference must not be empty")
	}
	ref := ImageRef{}
	rest := raw

	if before, after, found := cutAtLast(rest, "@"); found {
		ref.Digest = after
		rest = before
	}

	if before, after, found := cutAtLast(rest, ":"); found {
		if !strings.Contains(after, "/") {
			ref.Tag = after
			rest = before
		}
	}

	if ref.Tag == "" {
		ref.Tag = DefaultTag
	}

	parts := strings.Split(rest, "/")

	switch len(parts) {
	case 1:
		ref.Registry = DefaultRegistry
		ref.Repository = DefaultNamespace + "/" + parts[0]
	case 2:
		if isRegistryHost(parts[0]) {
			ref.Registry = parts[0]
			ref.Repository = parts[1]
		} else {
			ref.Registry = DefaultRegistry
			ref.Repository = parts[0] + "/" + parts[1]
		}
	default:
		ref.Registry = parts[0]
		ref.Repository = strings.Join(parts[1:], "/")
	}

	return ref, nil
}

func (ref ImageRef) String() string {
	var sb strings.Builder
	if ref.Registry != "" && ref.Registry != DefaultRegistry {
		sb.WriteString(ref.Registry)
		sb.WriteByte('/')
	}
	sb.WriteString(ref.Repository)
	if ref.Digest != "" {
		sb.WriteByte('@')
		sb.WriteString(ref.Digest)
	} else if ref.Tag != "" && ref.Tag != DefaultTag {
		sb.WriteByte(':')
		sb.WriteString(ref.Tag)
	}
	return sb.String()
}

func (ref ImageRef) FullName() string {
	name := ref.Registry + "/" + ref.Repository
	if ref.Digest != "" {
		return name + "@" + ref.Digest
	}
	return name + ":" + ref.Tag
}

func cutAtLast(s, sep string) (string, string, bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+1:], true
}

func isRegistryHost(host string) bool {
	if strings.Contains(host, ".") {
		return true
	}
	if host == "localhost" {
		return true
	}
	if strings.Contains(host, ":") {
		return true
	}
	return false
}
