package dockerfile

type Dockerfile struct {
	Instructions []Instruction
}

type Instruction interface {
	instruction()
}

type FromInstruction struct {
	Image    string
	Platform string
	Alias    string
}

func (*FromInstruction) instruction() {}

type KernelInstruction struct {
	Reference string
}

func (*KernelInstruction) instruction() {}

type RunInstruction struct {
	Command string
	Shell   bool
}

func (*RunInstruction) instruction() {}

type CopyInstruction struct {
	Sources     []string
	Destination string
	FromStage   string
}

func (*CopyInstruction) instruction() {}

type AddInstruction struct {
	Sources     []string
	Destination string
}

func (*AddInstruction) instruction() {}

type CmdInstruction struct {
	Command []string
	Shell   bool
}

func (*CmdInstruction) instruction() {}

type EntrypointInstruction struct {
	Command []string
	Shell   bool
}

func (*EntrypointInstruction) instruction() {}

type EnvInstruction struct {
	Key   string
	Value string
}

func (*EnvInstruction) instruction() {}

type WorkdirInstruction struct {
	Path string
}

func (*WorkdirInstruction) instruction() {}

type ExposeInstruction struct {
	Port     string
	Protocol string
}

func (*ExposeInstruction) instruction() {}

type VolumeInstruction struct {
	Path string
}

func (*VolumeInstruction) instruction() {}

type UserInstruction struct {
	User string
}

func (*UserInstruction) instruction() {}

type LabelInstruction struct {
	Key   string
	Value string
}

func (*LabelInstruction) instruction() {}

type ArgInstruction struct {
	Name    string
	Default string
}

func (*ArgInstruction) instruction() {}

type ShellInstruction struct {
	Shell []string
}

func (*ShellInstruction) instruction() {}

type CommentInstruction struct {
	Text string
}

func (*CommentInstruction) instruction() {}
