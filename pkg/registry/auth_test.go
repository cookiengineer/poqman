package registry

import (
	"testing"
)

func TestParseAuthHeader_DockerHub(t *testing.T) {
	header := `Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/alpine:pull"`

	auth, err := ParseAuthHeader(header)
	if err != nil {
		t.Fatalf("ParseAuthHeader: %v", err)
	}
	if auth.Realm != "https://auth.docker.io/token" {
		t.Errorf("expected realm https://auth.docker.io/token, got %s", auth.Realm)
	}
	if auth.Service != "registry.docker.io" {
		t.Errorf("expected service registry.docker.io, got %s", auth.Service)
	}
	if auth.Scope != "repository:library/alpine:pull" {
		t.Errorf("expected scope repository:library/alpine:pull, got %s", auth.Scope)
	}
}

func TestParseAuthHeader_NoRealm(t *testing.T) {
	header := `Bearer service="test",scope="test"`
	_, err := ParseAuthHeader(header)
	if err == nil {
		t.Error("expected error for missing realm")
	}
}

func TestParseAuthHeader_QuayStyle(t *testing.T) {
	header := `Bearer realm="https://quay.io/v2/auth",service="quay.io",scope="repository:org/app:pull"`

	auth, err := ParseAuthHeader(header)
	if err != nil {
		t.Fatalf("ParseAuthHeader: %v", err)
	}
	if auth.Realm != "https://quay.io/v2/auth" {
		t.Errorf("expected realm https://quay.io/v2/auth, got %s", auth.Realm)
	}
}

func TestParseAuthHeader_EmptyHeader(t *testing.T) {
	_, err := ParseAuthHeader("")
	if err == nil {
		t.Error("expected error for empty header")
	}
}
