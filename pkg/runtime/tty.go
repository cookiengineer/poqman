package runtime

import (
	"os"

	"golang.org/x/term"
)

type TerminalState struct {
	oldState *term.State
	fd       int
}

func MakeRawTerminal() (*TerminalState, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return &TerminalState{oldState: oldState, fd: fd}, nil
}

func (t *TerminalState) Restore() error {
	if t == nil || t.oldState == nil {
		return nil
	}
	return term.Restore(t.fd, t.oldState)
}
