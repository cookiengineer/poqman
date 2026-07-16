package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScan_File(t *testing.T) {
	tmp := t.TempDir()
	dockerfilePath := filepath.Join(tmp, "Dockerfile")
	content := "FROM alpine:latest\nRUN apk add nginx\n"
	os.WriteFile(dockerfilePath, []byte(content), 0o644)

	lines, err := Scan(dockerfilePath)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "FROM alpine:latest" {
		t.Errorf("unexpected line 0: %q", lines[0])
	}
}

func TestScan_NonexistentFile(t *testing.T) {
	_, err := Scan("/nonexistent/path/Dockerfile")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMergeContinuations_SingleLine(t *testing.T) {
	lines := []string{"FROM alpine:latest"}
	merged := mergeContinuations(lines)
	if len(merged) != 1 || merged[0] != "FROM alpine:latest" {
		t.Errorf("expected single line, got %v", merged)
	}
}

func TestMergeContinuations_EmptyInput(t *testing.T) {
	merged := mergeContinuations(nil)
	if len(merged) != 0 {
		t.Errorf("expected empty result, got %v", merged)
	}
}

func TestMergeContinuations_MultipleBackslash(t *testing.T) {
	lines := []string{
		"RUN apk add \\",
		"    nginx \\",
		"    curl",
		"FROM alpine",
	}
	merged := mergeContinuations(lines)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged lines, got %d", len(merged))
	}
	if !strings.HasPrefix(merged[0], "RUN apk add") {
		t.Errorf("unexpected line 0: %q", merged[0])
	}
	if merged[1] != "FROM alpine" {
		t.Errorf("unexpected line 1: %q", merged[1])
	}
}

func TestMergeContinuations_TrailingBackslash(t *testing.T) {
	lines := []string{
		"RUN echo hello \\",
	}
	merged := mergeContinuations(lines)
	if len(merged) != 1 {
		t.Fatalf("expected 1 line, got %d", len(merged))
	}
	if !strings.HasPrefix(merged[0], "RUN echo hello") {
		t.Errorf("unexpected: %q", merged[0])
	}
}

func TestMergeContinuations_EmptyAfterBackslash(t *testing.T) {
	lines := []string{
		"RUN echo hello \\",
		"",
		"FROM alpine",
	}
	merged := mergeContinuations(lines)
	if len(merged) < 1 {
		t.Fatal("expected at least 1 line")
	}
}

func TestMergeContinuations_CommentsAndBlanks(t *testing.T) {
	lines := []string{
		"# This is a comment",
		"",
		"   # indented comment",
		"FROM alpine:latest",
		"",
		"# another comment",
		"RUN echo hi",
	}
	merged := mergeContinuations(lines)
	if len(merged) != 2 {
		t.Fatalf("expected 2 non-comment lines, got %d: %v", len(merged), merged)
	}
}

func TestSplitLine_EdgeCases(t *testing.T) {
	tests := []struct {
		line        string
		wantKey     string
		wantRest    string
	}{
		{"FROM", "FROM", ""},
		{"FROM alpine:latest", "FROM", "alpine:latest"},
		{"RUN apk add nginx", "RUN", "apk add nginx"},
		{"ENV KEY=value", "ENV", "KEY=value"},
		{"CMD", "CMD", ""},
		{"   FROM   alpine   ", "FROM", "alpine"},
		{"KERNEL debian:6.1", "KERNEL", "debian:6.1"},
		{"LABEL key=value with spaces", "LABEL", "key=value with spaces"},
		{"\tFROM\talpine:latest", "FROM", "alpine:latest"},
	}

	for _, tt := range tests {
		key, rest := splitLine(tt.line)
		if key != tt.wantKey {
			t.Errorf("splitLine(%q) key = %q, want %q", tt.line, key, tt.wantKey)
		}
		if rest != tt.wantRest {
			t.Errorf("splitLine(%q) rest = %q, want %q", tt.line, rest, tt.wantRest)
		}
	}
}

func TestParseUnknownInstruction(t *testing.T) {
	lines := []string{"MAINTAINER nobody@example.com"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for unknown instruction")
	}
	if !strings.Contains(err.Error(), "unknown instruction") {
		t.Errorf("expected 'unknown instruction' in error, got: %v", err)
	}
}

func TestParseEmptyDockerfile(t *testing.T) {
	df, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(df.Instructions) != 0 {
		t.Errorf("expected 0 instructions, got %d", len(df.Instructions))
	}
}

func TestParseFrom_MissingArg(t *testing.T) {
	lines := []string{"FROM"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for FROM without image")
	}
}

func TestParseKernel_MissingArg(t *testing.T) {
	lines := []string{"KERNEL"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for KERNEL without reference")
	}
}

func TestParseRun_MissingArg(t *testing.T) {
	lines := []string{"RUN"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for RUN without command")
	}
}

func TestParseCopy_MissingArg(t *testing.T) {
	lines := []string{"COPY index.html"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for COPY without destination")
	}
}

func TestParseAdd_MissingArg(t *testing.T) {
	lines := []string{"ADD src.tar.gz"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for ADD without destination")
	}
}

func TestParseShell_InvalidForm(t *testing.T) {
	lines := []string{"SHELL /bin/sh -c"}
	_, err := Parse(lines)
	if err == nil {
		t.Error("expected error for SHELL without JSON array")
	}
}

func TestParseEntrypoint_Shell(t *testing.T) {
	lines := []string{"ENTRYPOINT /start.sh"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e := df.Instructions[0].(*EntrypointInstruction)
	if !e.Shell {
		t.Error("expected shell form for bare entrypoint")
	}
}

func TestParseCopy_MultipleSources(t *testing.T) {
	lines := []string{"COPY file1.txt file2.txt /dest/"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	c := df.Instructions[0].(*CopyInstruction)
	if len(c.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d: %v", len(c.Sources), c.Sources)
	}
	if c.Destination != "/dest/" {
		t.Errorf("expected /dest/, got %s", c.Destination)
	}
}

func TestParseAdd_MultipleSources(t *testing.T) {
	lines := []string{"ADD src1 src2 /dest/"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := df.Instructions[0].(*AddInstruction)
	if len(a.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(a.Sources))
	}
}

func TestParseFrom_LowercaseAs(t *testing.T) {
	lines := []string{"FROM alpine:latest as builder"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	from := df.Instructions[0].(*FromInstruction)
	if from.Alias != "builder" {
		t.Errorf("expected alias builder, got %s", from.Alias)
	}
}

func TestParseKernel_OCI(t *testing.T) {
	lines := []string{"KERNEL docker.io/poqman/kernel-debian:6.1.0-25"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	k := df.Instructions[0].(*KernelInstruction)
	if k.Reference != "docker.io/poqman/kernel-debian:6.1.0-25" {
		t.Errorf("unexpected reference: %s", k.Reference)
	}
}

func TestParseExpose_UDP(t *testing.T) {
	lines := []string{"EXPOSE 53/udp"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e := df.Instructions[0].(*ExposeInstruction)
	if e.Port != "53" {
		t.Errorf("expected 53, got %s", e.Port)
	}
	if e.Protocol != "udp" {
		t.Errorf("expected udp, got %s", e.Protocol)
	}
}

func TestParseArg_NoDefault(t *testing.T) {
	lines := []string{"ARG BUILDKIT_INLINE_CACHE"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a := df.Instructions[0].(*ArgInstruction)
	if a.Name != "BUILDKIT_INLINE_CACHE" {
		t.Errorf("unexpected name: %s", a.Name)
	}
	if a.Default != "" {
		t.Errorf("expected empty default, got %s", a.Default)
	}
}

func TestParseEnv_MultipleEquals(t *testing.T) {
	lines := []string{"ENV COMPOSE_FILE=docker-compose.yml:docker-compose.dev.yml"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	env := df.Instructions[0].(*EnvInstruction)
	if env.Key != "COMPOSE_FILE" {
		t.Errorf("expected COMPOSE_FILE, got %s", env.Key)
	}
	if env.Value != "docker-compose.yml:docker-compose.dev.yml" {
		t.Errorf("unexpected value: %s", env.Value)
	}
}

func TestParseVolume_ExecForm(t *testing.T) {
	lines := []string{`VOLUME ["/var/lib/mysql", "/var/log/mysql"]`}
	df, _ := Parse(lines)
	v := df.Instructions[0].(*VolumeInstruction)
	if v.Path != "/var/lib/mysql" {
		t.Errorf("expected /var/lib/mysql, got %s", v.Path)
	}
}

func TestParseUser_WithGroup(t *testing.T) {
	lines := []string{"USER nginx:nginx"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	u := df.Instructions[0].(*UserInstruction)
	if u.User != "nginx:nginx" {
		t.Errorf("expected nginx:nginx, got %s", u.User)
	}
}

func TestParseExec_SingleElement(t *testing.T) {
	parts, ok := parseExec(`["/usr/bin/supervisord"]`)
	if !ok {
		t.Fatal("expected valid exec form")
	}
	if len(parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(parts))
	}
}

func TestParseExec_EmptyArray(t *testing.T) {
	parts, ok := parseExec(`[]`)
	if !ok {
		t.Fatal("expected valid empty array")
	}
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

func TestParseExec_EscapedQuotes(t *testing.T) {
	parts, ok := parseExec(`["echo", "hello \"world\""]`)
	if !ok {
		t.Fatal("expected valid exec form with escaped quotes")
	}
	if len(parts) != 2 {
		t.Errorf("expected 2 parts, got %d", len(parts))
	}
}

func TestParseExec_Numbers(t *testing.T) {
	parts, ok := parseExec(`["1", "2", "3"]`)
	if !ok {
		t.Fatal("expected valid numeric strings in exec form")
	}
	if parts[0] != "1" || parts[1] != "2" || parts[2] != "3" {
		t.Errorf("unexpected parts: %v", parts)
	}
}

func TestParse_RunMultiWordCommand(t *testing.T) {
	lines := []string{"RUN apt-get update && apt-get install -y curl wget vim"}
	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := df.Instructions[0].(*RunInstruction)
	if !strings.Contains(r.Command, "apt-get update") {
		t.Errorf("expected apt-get update in command, got %s", r.Command)
	}
}

func TestParse_FullDockerfile(t *testing.T) {
	lines := []string{
		"FROM --platform=linux/amd64 alpine:latest AS base",
		`KERNEL "debian:6.1.0-25-amd64"`,
		"LABEL maintainer=\"test@example.com\"",
		"ARG VERSION=latest",
		"ENV APP_HOME=/app",
		"WORKDIR $APP_HOME",
		"RUN apk add --no-cache nginx",
		"COPY index.html /usr/share/nginx/html/",
		"COPY --from=base /etc/ssl/certs /etc/ssl/certs",
		"ADD https://example.com/config.json /etc/config.json",
		"EXPOSE 80/tcp",
		"EXPOSE 443/tcp",
		"VOLUME /var/lib/nginx",
		"USER nginx",
		`ENTRYPOINT ["/docker-entrypoint.sh"]`,
		`CMD ["nginx", "-g", "daemon off;"]`,
		"SHELL [\"/bin/ash\", \"-eo\", \"pipefail\", \"-c\"]",
	}

	df, err := Parse(lines)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(df.Instructions) != 17 {
		t.Fatalf("expected 17 instructions, got %d", len(df.Instructions))
	}

	typeChecks := map[int]string{
		0:  "*FromInstruction",
		1:  "*KernelInstruction",
		2:  "*LabelInstruction",
		3:  "*ArgInstruction",
		4:  "*EnvInstruction",
		5:  "*WorkdirInstruction",
		6:  "*RunInstruction",
		7:  "*CopyInstruction",
		8:  "*CopyInstruction",
		9:  "*AddInstruction",
		10: "*ExposeInstruction",
		11: "*ExposeInstruction",
		12: "*VolumeInstruction",
		13: "*UserInstruction",
		14: "*EntrypointInstruction",
		15: "*CmdInstruction",
		16: "*ShellInstruction",
	}

	for i, instr := range df.Instructions {
		expected := typeChecks[i]
		got := typeName(instr)
		if got != expected {
			t.Errorf("instruction %d: expected %s, got %s", i, expected, got)
		}
	}

	// Verify specific values
	from := df.Instructions[0].(*FromInstruction)
	if from.Platform != "linux/amd64" {
		t.Errorf("expected platform linux/amd64, got %s", from.Platform)
	}
	if from.Alias != "base" {
		t.Errorf("expected alias base, got %s", from.Alias)
	}

	cmd := df.Instructions[15].(*CmdInstruction)
	if cmd.Shell {
		t.Error("expected exec form for CMD")
	}
	if len(cmd.Command) != 3 {
		t.Errorf("expected 3 args, got %d", len(cmd.Command))
	}

	shell := df.Instructions[16].(*ShellInstruction)
	if len(shell.Shell) != 4 {
		t.Errorf("expected 4 shell parts, got %d", len(shell.Shell))
	}
}

func typeName(instr Instruction) string {
	switch instr.(type) {
	case *FromInstruction:
		return "*FromInstruction"
	case *KernelInstruction:
		return "*KernelInstruction"
	case *RunInstruction:
		return "*RunInstruction"
	case *CopyInstruction:
		return "*CopyInstruction"
	case *AddInstruction:
		return "*AddInstruction"
	case *CmdInstruction:
		return "*CmdInstruction"
	case *EntrypointInstruction:
		return "*EntrypointInstruction"
	case *EnvInstruction:
		return "*EnvInstruction"
	case *WorkdirInstruction:
		return "*WorkdirInstruction"
	case *ExposeInstruction:
		return "*ExposeInstruction"
	case *VolumeInstruction:
		return "*VolumeInstruction"
	case *UserInstruction:
		return "*UserInstruction"
	case *LabelInstruction:
		return "*LabelInstruction"
	case *ArgInstruction:
		return "*ArgInstruction"
	case *ShellInstruction:
		return "*ShellInstruction"
	case *CommentInstruction:
		return "*CommentInstruction"
	}
	return "unknown"
}

func TestInstrMethod_AllTypes(t *testing.T) {
	instrs := []Instruction{
		&FromInstruction{},
		&KernelInstruction{},
		&RunInstruction{},
		&CopyInstruction{},
		&AddInstruction{},
		&CmdInstruction{},
		&EntrypointInstruction{},
		&EnvInstruction{},
		&WorkdirInstruction{},
		&ExposeInstruction{},
		&VolumeInstruction{},
		&UserInstruction{},
		&LabelInstruction{},
		&ArgInstruction{},
		&ShellInstruction{},
		&CommentInstruction{},
	}

	for _, instr := range instrs {
		instr.instruction()
	}
}
