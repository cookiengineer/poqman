package cli

import (
	"testing"
)

func TestRegisterExec(t *testing.T) {
	r := NewRouter()
	RegisterExec(r)

	if _, ok := r.commands["exec"]; !ok {
		t.Error("expected 'exec' command to be registered")
	}

	cmd := r.commands["exec"]
	if cmd.Name != "exec" {
		t.Errorf("expected name 'exec', got %s", cmd.Name)
	}
	if cmd.Description == "" {
		t.Error("expected non-empty description")
	}
}
