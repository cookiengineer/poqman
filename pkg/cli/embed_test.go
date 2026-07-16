package cli

import (
	"runtime"
	"strings"
	"testing"
)

func TestInitBinary_FallbackOnHostArch(t *testing.T) {
	data := InitBinary(runtime.GOARCH)
	if len(data) == 0 {
		t.Error("InitBinary should never return empty data")
	}
}

func TestInitBinary_EmptyArch(t *testing.T) {
	data := InitBinary("")
	if len(data) == 0 {
		t.Error("InitBinary with empty arch should return data")
	}
}

func TestInitBinary_UnknownArch(t *testing.T) {
	data := InitBinary("mips64")
	if len(data) == 0 {
		t.Error("InitBinary for unknown arch should return fallback (not empty)")
	}
}

func TestDefaultInitScript_Content(t *testing.T) {
	data := defaultInitBinary()
	content := string(data)
	if len(data) == 0 {
		t.Fatal("defaultInitBinary must return fallback shell script")
	}
	if !strings.HasPrefix(content, "#!/bin/sh") {
		t.Error("fallback init should be a shell script starting with #!/bin/sh")
	}
	requiredMounts := []string{
		"mount -t proc proc /proc",
		"mount -t sysfs sys /sys",
		"mount -t devtmpfs dev /dev",
	}
	for _, m := range requiredMounts {
		if !strings.Contains(content, m) {
			t.Errorf("fallback init must contain mount command: %q", m)
		}
	}
	if !strings.Contains(content, "ip addr add") {
		t.Error("fallback init should configure network")
	}
	if !strings.Contains(content, "poqman.cmd=") {
		t.Error("fallback init should parse poqman.cmd= from kernel cmdline")
	}
	if !strings.Contains(content, "exec /bin/sh -c") {
		t.Error("fallback init must exec the CMD")
	}
}

func TestAgentBinary_FallbackOnHostArch(t *testing.T) {
	data := AgentBinary(runtime.GOARCH)
	if len(data) == 0 {
		t.Log("AgentBinary returned empty (no agent binary embedded, no fallback script — not critical)")
	}
}

func TestDefaultInitBinary_IsNotEmpty(t *testing.T) {
	data := defaultInitBinary()
	if len(data) == 0 {
		t.Error("defaultInitBinary must return the fallback shell script")
	}
}

func TestDefaultAgentBinary_IsEmptyByDesign(t *testing.T) {
	data := defaultAgentBinary()
	if len(data) != 0 {
		t.Error("defaultAgentBinary returns empty by design (no fallback for agent)")
	}
}

func TestInitBinary_ConsistentOutput(t *testing.T) {
	d1 := InitBinary("arm64")
	d2 := InitBinary("arm64")
	if len(d1) != len(d2) {
		t.Error("InitBinary should return consistent results")
	}
}

func TestAgentBinary_ConsistentOutput(t *testing.T) {
	d1 := AgentBinary("arm64")
	d2 := AgentBinary("arm64")
	if len(d1) != len(d2) {
		t.Error("AgentBinary should return consistent results")
	}
}

func TestInitBinary_DataIsExecutable(t *testing.T) {
	data := InitBinary("amd64")
	if len(data) == 0 {
		t.Fatal("InitBinary must return data")
	}
	if strings.HasPrefix(string(data), "#!/bin/sh") {
		t.Log("using shell script fallback init")
	} else if strings.HasPrefix(string(data), "\x7fELF") {
		t.Log("using embedded Go ELF binary")
	}
}
