package dockerfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type scanner struct {
	lines   []string
	pos     int
	aliases map[string]string
}

func Scan(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Dockerfile: %w", err)
	}
	defer file.Close()

	var rawLines []string
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		rawLines = append(rawLines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read Dockerfile: %w", err)
	}

	return mergeContinuations(rawLines), nil
}

func mergeContinuations(lines []string) []string {
	var merged []string
	var current strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmed, "\\") {
			current.WriteString(strings.TrimSuffix(trimmed, "\\"))
			current.WriteString(" ")
			continue
		}
		current.WriteString(trimmed)
		merged = append(merged, current.String())
		current.Reset()
	}

	if current.Len() > 0 {
		merged = append(merged, current.String())
	}

	var result []string
	for _, line := range merged {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result = append(result, line)
	}

	return result
}

func Parse(lines []string) (*Dockerfile, error) {
	s := &scanner{
		lines:   lines,
		pos:     0,
		aliases: make(map[string]string),
	}

	var instructions []Instruction

	for s.pos < len(s.lines) {
		line := s.lines[s.pos]
		s.pos++

		instr, err := s.parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", s.pos, err)
		}
		if instr != nil {
			if from, ok := instr.(*FromInstruction); ok && from.Alias != "" {
				s.aliases[strings.ToLower(from.Alias)] = from.Image
			}
			if copyInstr, ok := instr.(*CopyInstruction); ok && copyInstr.FromStage != "" {
				if resolved, exists := s.aliases[strings.ToLower(copyInstr.FromStage)]; exists {
					copyInstr.FromStage = resolved
				}
			}
			instructions = append(instructions, instr)
		}
	}

	return &Dockerfile{Instructions: instructions}, nil
}

func (s *scanner) parseLine(line string) (Instruction, error) {
	keyword, rest := splitLine(line)
	keyword = strings.ToUpper(keyword)

	switch keyword {
	case "FROM":
		return parseFrom(rest)
	case "KERNEL":
		return parseKernel(rest)
	case "RUN":
		return parseRun(rest)
	case "COPY":
		return parseCopy(rest)
	case "ADD":
		return parseAdd(rest)
	case "CMD":
		return parseCmd(rest)
	case "ENTRYPOINT":
		return parseEntrypoint(rest)
	case "ENV":
		return parseEnv(rest)
	case "WORKDIR":
		return parseWorkdir(rest)
	case "EXPOSE":
		return parseExpose(rest)
	case "VOLUME":
		return parseVolume(rest)
	case "USER":
		return parseUser(rest)
	case "LABEL":
		return parseLabel(rest)
	case "ARG":
		return parseArg(rest)
	case "SHELL":
		return parseShell(rest)
	case "HEALTHCHECK":
		return parseHealthCheck(rest)
	case "#":
		return nil, nil
	}

	return nil, fmt.Errorf("unknown instruction: %s", keyword)
}

func splitLine(line string) (string, string) {
	line = strings.TrimSpace(line)

	if idx := strings.IndexAny(line, " \t"); idx >= 0 {
		keyword := line[:idx]
		rest := strings.TrimSpace(line[idx+1:])
		return keyword, rest
	}

	return line, ""
}

func parseFrom(rest string) (*FromInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("FROM requires an image")
	}

	instr := &FromInstruction{}

	idxPlatform := strings.Index(rest, "--platform=")
	if idxPlatform >= 0 {
		end := strings.IndexAny(rest[idxPlatform:], " \t")
		if end >= 0 {
			instr.Platform = rest[idxPlatform+11 : idxPlatform+end]
		} else {
			instr.Platform = rest[idxPlatform+11:]
		}
	}

	cleanRest := rest
	if strings.HasPrefix(cleanRest, "--platform=") {
		spaceIdx := strings.Index(cleanRest, " ")
		if spaceIdx >= 0 {
			cleanRest = strings.TrimSpace(cleanRest[spaceIdx:])
		}
	}

	idx := strings.Index(cleanRest, " AS ")
	if idx < 0 {
		idx = strings.Index(cleanRest, " as ")
	}
	if idx >= 0 {
		instr.Image = strings.TrimSpace(cleanRest[:idx])
		instr.Alias = strings.TrimSpace(cleanRest[idx+4:])
	} else {
		instr.Image = strings.TrimSpace(cleanRest)
	}

	if instr.Image == "" {
		return nil, fmt.Errorf("FROM requires an image name")
	}

	return instr, nil
}

func parseKernel(rest string) (*KernelInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("KERNEL requires a reference")
	}

	ref := rest
	if strings.HasPrefix(ref, "\"") && strings.HasSuffix(ref, "\"") {
		ref = ref[1 : len(ref)-1]
	}

	return &KernelInstruction{Reference: ref}, nil
}

func parseRun(rest string) (*RunInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("RUN requires a command")
	}

	if exec, ok := parseExec(rest); ok {
		return &RunInstruction{Command: strings.Join(exec, " "), Shell: false}, nil
	}

	return &RunInstruction{Command: rest, Shell: true}, nil
}

func parseCopy(rest string) (*CopyInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("COPY requires source and destination")
	}

	instr := &CopyInstruction{}

	parts := strings.Fields(rest)
	var filtered []string
	for _, part := range parts {
		if strings.HasPrefix(part, "--from=") {
			instr.FromStage = strings.TrimPrefix(part, "--from=")
			continue
		}
		filtered = append(filtered, part)
	}

	if len(filtered) < 2 {
		return nil, fmt.Errorf("COPY requires at least source and destination")
	}

	instr.Sources = filtered[:len(filtered)-1]
	instr.Destination = filtered[len(filtered)-1]

	return instr, nil
}

func parseAdd(rest string) (*AddInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("ADD requires source and destination")
	}

	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return nil, fmt.Errorf("ADD requires at least source and destination")
	}

	return &AddInstruction{
		Sources:     parts[:len(parts)-1],
		Destination: parts[len(parts)-1],
	}, nil
}

func parseCmd(rest string) (*CmdInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("CMD requires a command")
	}

	if exec, ok := parseExec(rest); ok {
		return &CmdInstruction{Command: exec, Shell: false}, nil
	}

	return &CmdInstruction{Command: strings.Fields(rest), Shell: true}, nil
}

func parseEntrypoint(rest string) (*EntrypointInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("ENTRYPOINT requires a command")
	}

	if exec, ok := parseExec(rest); ok {
		return &EntrypointInstruction{Command: exec, Shell: false}, nil
	}

	return &EntrypointInstruction{Command: strings.Fields(rest), Shell: true}, nil
}

func parseEnv(rest string) (*EnvInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("ENV requires key=value")
	}

	parts := strings.SplitN(rest, "=", 2)
	if len(parts) == 2 {
		return &EnvInstruction{Key: parts[0], Value: parts[1]}, nil
	}

	parts = strings.Fields(rest)
	if len(parts) >= 2 {
		return &EnvInstruction{Key: parts[0], Value: strings.Join(parts[1:], " ")}, nil
	}

	return nil, fmt.Errorf("ENV requires key and value")
}

func parseWorkdir(rest string) (*WorkdirInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("WORKDIR requires a path")
	}
	return &WorkdirInstruction{Path: rest}, nil
}

func parseExpose(rest string) (*ExposeInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("EXPOSE requires a port")
	}

	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 2 {
		return &ExposeInstruction{Port: parts[0], Protocol: parts[1]}, nil
	}
	return &ExposeInstruction{Port: parts[0], Protocol: "tcp"}, nil
}

func parseVolume(rest string) (*VolumeInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("VOLUME requires a path")
	}

	if exec, ok := parseExec(rest); ok {
		return &VolumeInstruction{Path: exec[0]}, nil
	}

	return &VolumeInstruction{Path: rest}, nil
}

func parseUser(rest string) (*UserInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("USER requires a username")
	}
	return &UserInstruction{User: rest}, nil
}

func parseLabel(rest string) (*LabelInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("LABEL requires key=value")
	}

	parts := strings.SplitN(rest, "=", 2)
	if len(parts) == 2 {
		return &LabelInstruction{Key: parts[0], Value: parts[1]}, nil
	}

	return nil, fmt.Errorf("LABEL requires key=value")
}

func parseArg(rest string) (*ArgInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("ARG requires a name")
	}

	parts := strings.SplitN(rest, "=", 2)
	if len(parts) == 2 {
		return &ArgInstruction{Name: parts[0], Default: parts[1]}, nil
	}

	return &ArgInstruction{Name: rest}, nil
}

func parseShell(rest string) (*ShellInstruction, error) {
	if rest == "" {
		return nil, fmt.Errorf("SHELL requires a command array")
	}

	if exec, ok := parseExec(rest); ok {
		return &ShellInstruction{Shell: exec}, nil
	}

	return nil, fmt.Errorf("SHELL requires JSON array form")
}

func parseHealthCheck(rest string) (*HealthCheckInstruction, error) {
	if rest == "" || strings.ToUpper(rest) == "NONE" {
		return &HealthCheckInstruction{}, nil
	}

	h := &HealthCheckInstruction{}

	fields := strings.Fields(rest)
	cmdStart := 0
	for i, f := range fields {
		upper := strings.ToUpper(f)
		switch {
		case strings.HasPrefix(upper, "--INTERVAL="):
			h.Interval = strings.SplitN(f, "=", 2)[1]
		case strings.HasPrefix(upper, "--TIMEOUT="):
			h.Timeout = strings.SplitN(f, "=", 2)[1]
		case strings.HasPrefix(upper, "--RETRIES="):
			fmt.Sscanf(strings.SplitN(f, "=", 2)[1], "%d", &h.Retries)
		case strings.HasPrefix(upper, "--START-PERIOD="):
			h.StartPeriod = strings.SplitN(f, "=", 2)[1]
		default:
			cmdStart = i
			goto parseCmd
		}
	}
parseCmd:

	if cmdStart < len(fields) && strings.ToUpper(fields[cmdStart]) == "CMD" {
		cmdStart++
	}

	restCmd := strings.Join(fields[cmdStart:], " ")
	if restCmd == "" {
		return nil, fmt.Errorf("HEALTHCHECK requires a command")
	}

	if exec, ok := parseExec(restCmd); ok {
		h.Command = exec
	} else {
		h.Command = []string{"CMD-SHELL", restCmd}
	}

	return h, nil
}

func parseExec(rest string) ([]string, bool) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
		return nil, false
	}

	var parts []string
	if err := json.Unmarshal([]byte(rest), &parts); err != nil {
		return nil, false
	}

	return parts, true
}
