package cli

import (
	"testing"
)

func TestRegisterBuild(t *testing.T) {
	r := NewRouter()
	RegisterBuild(r)

	if _, ok := r.commands["build"]; !ok {
		t.Error("expected 'build' command to be registered")
	}

	cmd := r.commands["build"]
	if cmd.Name != "build" {
		t.Errorf("expected name 'build', got %s", cmd.Name)
	}
	if cmd.Description == "" {
		t.Error("expected non-empty description")
	}

	fs := cmd.FlagSet
	if fs.Lookup("t") == nil {
		t.Error("expected -t flag")
	}
	if fs.Lookup("f") == nil {
		t.Error("expected -f flag")
	}
	if fs.Lookup("platform") == nil {
		t.Error("expected --platform flag")
	}
}
