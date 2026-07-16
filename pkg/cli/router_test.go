package cli

import (
	"testing"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if len(r.commands) != 0 {
		t.Errorf("expected empty commands, got %d", len(r.commands))
	}
}

func TestRouter_Register(t *testing.T) {
	r := NewRouter()
	r.Register(&Command{
		Name:        "test",
		Description: "test command",
		Usage:       "[args]",
		Run:         func(args []string) error { return nil },
	})

	if len(r.commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(r.commands))
	}
	if _, ok := r.commands["test"]; !ok {
		t.Error("expected 'test' command to be registered")
	}
}

func TestRouter_DispatchUnknown(t *testing.T) {
	r := NewRouter()
	err := r.Dispatch([]string{"unknown"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRouter_DispatchHelp(t *testing.T) {
	r := NewRouter()
	r.Register(&Command{
		Name:        "test",
		Description: "a test",
		Usage:       "[args]",
		Run:         func(args []string) error { return nil },
	})

	err := r.Dispatch([]string{"help", "test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = r.Dispatch([]string{"help"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = r.Dispatch([]string{"--help"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRouter_DispatchEmptyArgs(t *testing.T) {
	r := NewRouter()
	err := r.Dispatch(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRouter_DispatchCommand(t *testing.T) {
	r := NewRouter()
	executed := false
	r.Register(&Command{
		Name:        "test",
		Description: "test command",
		Usage:       "[args]",
		Run: func(args []string) error {
			executed = true
			if len(args) != 1 || args[0] != "hello" {
				t.Errorf("expected args [hello], got %v", args)
			}
			return nil
		},
	})

	err := r.Dispatch([]string{"test", "hello"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("expected command to be executed")
	}
}

func TestFormatImageName(t *testing.T) {
	tests := []struct {
		repo string
		tag  string
		want string
	}{
		{"docker.io/library/alpine", "latest", "alpine"},
		{"library/nginx", "1.25", "nginx"},
		{"myrepo", "v1", "myrepo"},
	}

	for _, tt := range tests {
		if got := FormatImageName(tt.repo, tt.tag); got != tt.want {
			t.Errorf("FormatImageName(%q, %q) = %q, want %q", tt.repo, tt.tag, got, tt.want)
		}
	}
}

func TestRouter_FlagSetAutoCreated(t *testing.T) {
	r := NewRouter()
	r.Register(&Command{
		Name:        "test",
		Description: "desc",
	})

	if r.commands["test"].FlagSet == nil {
		t.Error("expected FlagSet to be auto-created")
	}
}
