package cli

import (
	"testing"
)

func TestExecFlagsExist(t *testing.T) {
	r := NewRouter()
	RegisterExec(r)

	cmd := r.commands["exec"]
	fs := cmd.FlagSet
	if fs.Lookup("workdir") == nil {
		t.Error("expected --workdir flag")
	}
}

func TestExecRequiresArgs(t *testing.T) {
	r := NewRouter()
	RegisterExec(r)
	cmd := r.commands["exec"]

	err := cmd.Run([]string{})
	if err == nil {
		t.Error("expected error when no args provided")
	}
}

func TestStrSliceFlag_Append(t *testing.T) {
	var s strSliceFlag
	s.Set("KEY=VALUE")
	s.Set("DEBUG=1")

	if len(s) != 2 {
		t.Errorf("expected 2 entries, got %d", len(s))
	}
	if s[0] != "KEY=VALUE" {
		t.Errorf("expected KEY=VALUE, got %s", s[0])
	}
	if s[1] != "DEBUG=1" {
		t.Errorf("expected DEBUG=1, got %s", s[1])
	}
}

func TestStrSliceFlag_String(t *testing.T) {
	var s strSliceFlag
	s.Set("A=1")
	s.Set("B=2")
	got := s.String()
	if got != "[A=1 B=2]" {
		t.Errorf("unexpected string: %s", got)
	}
}
