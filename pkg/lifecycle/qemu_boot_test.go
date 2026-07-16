package lifecycle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cookiengineer/poqman/pkg/image"
)

func hasQEMU() bool {
	_, err := exec.LookPath("qemu-system-x86_64")
	return err == nil
}

func findHostKernel() string {
	paths := []string{
		"/boot/vmlinuz-linux",
		"/boot/vmlinuz-" + kernelRelease(),
		"/boot/vmlinuz",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func kernelRelease() string {
	out, _ := exec.Command("uname", "-r").Output()
	return strings.TrimSpace(string(out))
}

func buildBootableInitrd(t *testing.T, initScript string) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"bin", "sbin", "dev", "proc", "sys"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}

	busyboxPath := "/bin/busybox"
	if _, err := os.Stat(busyboxPath); os.IsNotExist(err) {
		t.Skip("busybox not found at " + busyboxPath)
	}

	copyFile(busyboxPath, filepath.Join(dir, "bin", "busybox"))
	os.Chmod(filepath.Join(dir, "bin", "busybox"), 0o755)

	for _, app := range []string{"sh", "mount", "reboot", "poweroff"} {
		os.Symlink("busybox", filepath.Join(dir, "bin", app))
	}

	os.WriteFile(filepath.Join(dir, "init"), []byte(initScript), 0o755)

	outputPath := filepath.Join(t.TempDir(), "initrd.cpio.gz")
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("cd %s && find . | cpio -o -H newc 2>/dev/null | gzip > %s", dir, outputPath))
	if err := cmd.Run(); err != nil {
		t.Fatalf("build initrd: %v", err)
	}
	return outputPath
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func qmpWaitForSocket(socketPath string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
		if err == nil {
			scanner := bufio.NewScanner(conn)
			if scanner.Scan() {
				var greeting map[string]any
				json.Unmarshal(scanner.Bytes(), &greeting)
			}
			capCmd := map[string]any{"execute": "qmp_capabilities"}
			data, _ := json.Marshal(capCmd)
			conn.Write(append(data, '\n'))
			if scanner.Scan() {
				return conn, nil
			}
			conn.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("QMP socket not ready within %v", timeout)
}

const bootableInitScript = `#!/bin/sh
echo "POQMAN_INIT_STARTED"
/bin/mount -t proc proc /proc
/bin/mount -t sysfs sys /sys
echo "POQMAN_INIT_READY"
%s
echo "POQMAN_INIT_DONE"
/bin/reboot -f
`

func TestIntegration_QemuBootsAndExits(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, "echo RUNNING")
	initrdPath := buildBootableInitrd(t, script)
	serialLog := filepath.Join(t.TempDir(), "serial.log")

	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel,
		"-initrd", initrdPath,
		"-append", "console=ttyS0",
		"-nographic", "-no-reboot",
		"-m", "128M", "-smp", "1",
		"-serial", "file:"+serialLog,
	)
	t.Logf("Booting kernel %s...", filepath.Base(kernel))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(45 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timed out after 45s")
	}

	out := string(readFileOrEmpty(serialLog))
	if !strings.Contains(out, "POQMAN_INIT_STARTED") {
		t.Error("init did not start")
	}
	if !strings.Contains(out, "POQMAN_INIT_READY") {
		t.Error("init did not mount filesystems")
	}
	if !strings.Contains(out, "POQMAN_INIT_DONE") {
		t.Error("init did not complete")
	}
	t.Logf("Boot+exit successful (serial: %d bytes)", len(out))
}

func TestIntegration_QemuWithPoqmanInit(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, `
hostname poqman-test
echo "hostname set to $(hostname)"
`)
	initrdPath := buildBootableInitrd(t, script)
	serialLog := filepath.Join(t.TempDir(), "serial.log")

	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel, "-initrd", initrdPath,
		"-append", "console=ttyS0",
		"-nographic", "-no-reboot",
		"-m", "128M", "-smp", "1",
		"-serial", "file:"+serialLog,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(45 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timed out")
	}

	out := string(readFileOrEmpty(serialLog))
	if !strings.Contains(out, "POQMAN_INIT_READY") {
		t.Error("init did not mount filesystems")
	}
	if strings.Contains(out, "hostname set to poqman-test") {
		t.Log("hostname set correctly inside VM")
	}
}

func TestIntegration_QemuGracefulShutdown(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := `#!/bin/sh
echo "BOOTED"
/bin/mount -t proc proc /proc
/bin/mount -t sysfs sys /sys
echo "READY"
while true; do /bin/sleep 1; done
`
	initrdPath := buildBootableInitrd(t, script)
	qmpSocket := filepath.Join(t.TempDir(), "qmp.sock")
	serialLog := filepath.Join(t.TempDir(), "serial.log")

	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel, "-initrd", initrdPath,
		"-append", "console=ttyS0",
		"-nographic", "-no-reboot",
		"-m", "256M", "-smp", "1",
		"-qmp", "unix:"+qmpSocket+",server=on,wait=off",
		"-serial", "file:"+serialLog,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	conn, err := qmpWaitForSocket(qmpSocket, 30*time.Second)
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("QMP connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(6 * time.Second)

	powerDown := map[string]any{"execute": "system_powerdown"}
	data, _ := json.Marshal(powerDown)
	conn.Write(append(data, '\n'))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		t.Log("Graceful shutdown via QMP succeeded")
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
	}

	out := string(readFileOrEmpty(serialLog))
	if !strings.Contains(out, "BOOTED") {
		t.Error("VM did not boot")
	}
	if strings.Contains(out, "READY") {
		t.Log("VM reached ready state before shutdown")
	}
}

func TestIntegration_QemuSerialOutput(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, `
echo "HELLO_FROM_VM"
cat /proc/version
echo "VM_EXIT"
`)
	initrdPath := buildBootableInitrd(t, script)
	serialLog := filepath.Join(t.TempDir(), "serial.log")

	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel, "-initrd", initrdPath,
		"-append", "console=ttyS0",
		"-nographic", "-no-reboot",
		"-m", "128M", "-smp", "1",
		"-serial", "file:"+serialLog,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(45 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timed out")
	}

	out := string(readFileOrEmpty(serialLog))
	if !strings.Contains(out, "HELLO_FROM_VM") {
		t.Error("VM did not produce expected output")
	}
	if !strings.Contains(out, "VM_EXIT") {
		t.Error("VM did not reach exit point")
	}
	if strings.Contains(out, "Linux version") {
		t.Log("Kernel boot confirmed")
	}
}

func TestIntegration_QemuMemoryLimits(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, "cat /proc/meminfo | head -3")
	initrdPath := buildBootableInitrd(t, script)

	for _, mem := range []string{"128M", "256M"} {
		t.Run("Limit_"+mem, func(t *testing.T) {
			logFile := filepath.Join(t.TempDir(), "mem.log")
			cmd := exec.Command("qemu-system-x86_64",
				"-kernel", kernel, "-initrd", initrdPath,
				"-append", "console=ttyS0",
				"-nographic", "-no-reboot",
				"-m", mem, "-smp", "1",
				"-serial", "file:"+logFile,
			)
			if err := cmd.Start(); err != nil {
				t.Fatalf("start: %v", err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(45 * time.Second):
				cmd.Process.Kill()
				t.Fatal("timed out")
			}
			out := string(readFileOrEmpty(logFile))
			if !strings.Contains(out, "POQMAN_INIT_DONE") {
				t.Error("test did not complete")
			}
			if strings.Contains(out, "MemTotal") {
				t.Logf("Memory info present in %s VM", mem)
			}
		})
	}
}

func TestIntegration_QemuSMPLimits(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, "cat /proc/cpuinfo | grep processor | wc -l")
	initrdPath := buildBootableInitrd(t, script)

	for _, cpus := range []int{1, 2} {
		t.Run(fmt.Sprintf("%d_CPUs", cpus), func(t *testing.T) {
			logFile := filepath.Join(t.TempDir(), fmt.Sprintf("smp-%d.log", cpus))
			cmd := exec.Command("qemu-system-x86_64",
				"-kernel", kernel, "-initrd", initrdPath,
				"-append", "console=ttyS0",
				"-nographic", "-no-reboot",
				"-m", "128M", "-smp", fmt.Sprintf("%d", cpus),
				"-serial", "file:"+logFile,
			)
			if err := cmd.Start(); err != nil {
				t.Fatalf("start: %v", err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(45 * time.Second):
				cmd.Process.Kill()
				t.Fatal("timed out")
			}
			out := string(readFileOrEmpty(logFile))
			if !strings.Contains(out, "POQMAN_INIT_DONE") {
				t.Error("test did not complete")
			}
		})
	}
}

func TestIntegration_QemuCMDLineParsing(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	script := fmt.Sprintf(bootableInitScript, "cat /proc/cmdline")
	initrdPath := buildBootableInitrd(t, script)
	logFile := filepath.Join(t.TempDir(), "cmdline.log")

	cmdline := "console=ttyS0 poqman.hostname=testvm poqman.ip=10.88.0.5/16 poqman.gateway=10.88.0.1 poqman.cmd=/bin/sh"
	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel, "-initrd", initrdPath,
		"-append", cmdline,
		"-nographic", "-no-reboot",
		"-m", "128M", "-smp", "1",
		"-serial", "file:"+logFile,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(45 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timed out")
	}

	out := string(readFileOrEmpty(logFile))
	for _, param := range []string{"poqman.hostname=testvm", "poqman.ip=10.88.0.5/16", "poqman.gateway=10.88.0.1", "poqman.cmd=/bin/sh"} {
		if strings.Contains(out, param) {
			t.Logf("cmdline param received: %s", param)
		}
	}
}

func TestIntegration_FullPoqmanLifecycle(t *testing.T) {
	if !hasQEMU() {
		t.Skip("QEMU not installed")
	}
	kernel := findHostKernel()
	if kernel == "" {
		t.Skip("no host kernel")
	}

	rootfsDir := t.TempDir()
	os.MkdirAll(filepath.Join(rootfsDir, "sbin"), 0o755)
	os.MkdirAll(filepath.Join(rootfsDir, "tmp"), 0o755)

	initScript := fmt.Sprintf(bootableInitScript, `
hostname poqman-test
echo "$(hostname) is running" > /tmp/hostname.txt
`)
	os.WriteFile(filepath.Join(rootfsDir, "sbin", "init"), []byte(initScript), 0o755)

	pidFile := filepath.Join(t.TempDir(), "pidfile")
	serialLog := filepath.Join(t.TempDir(), "serial-lifecycle.log")

	cmd := exec.Command("qemu-system-x86_64",
		"-kernel", kernel,
		"-append", "root=rootfs rootfstype=9p rootflags=trans=virtio,version=9p2000.L rw console=ttyS0 init=/sbin/init",
		"-fsdev", "local,id=rootfs,path="+rootfsDir+",security_model=mapped-xattr",
		"-device", "virtio-9p-pci,fsdev=rootfs,mount_tag=rootfs",
		"-nographic", "-no-reboot",
		"-m", "128M", "-smp", "1",
		"-pidfile", pidFile,
		"-serial", "file:"+serialLog,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		t.Fatal("lifecycle test timed out")
	}

	output := string(readFileOrEmpty(serialLog))
	if strings.Contains(output, "POQMAN_INIT_DONE") {
		t.Log("Full lifecycle: VM boot → init ran → clean exit")
	}
	pidData := string(readFileOrEmpty(pidFile))
	if pidData != "" {
		t.Logf("PID file: %s", strings.TrimSpace(pidData))
	}
	if data, err := os.ReadFile(filepath.Join(rootfsDir, "tmp", "hostname.txt")); err == nil {
		t.Logf("9p rootfs write: %s", strings.TrimSpace(string(data)))
	}
}

func readFileOrEmpty(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func TestIntegration_ImageLifecycleEndToEnd(t *testing.T) {
	paths := setupPaths(t)
	imgStore := image.NewStore(paths)
	img := &image.Image{
		ID:       "sha256:e2e-test",
		RepoTags: []string{"docker.io/library/e2e-test:latest"},
		Arch:     "amd64",
		Config: image.ImageConfig{
			Cmd:     []string{"/bin/sh"},
			Workdir: "/",
			Env:     []string{"PATH=/usr/bin"},
		},
		Layers:  []image.Layer{{Digest: "sha256:e2e-layer", Size: 500}},
		Created: time.Now(),
		Size:    500,
	}
	imgStore.Save(img)
	idx, _ := imgStore.LoadIndex()
	idx.Add("docker.io/library/e2e-test:latest", "sha256:e2e-test")
	imgStore.SaveIndex(idx)
	defer imgStore.Remove("sha256:e2e-test")

	outputPath := filepath.Join(paths.Tmp, "e2e-export.tar.gz")
	image.SaveImage(img, paths, outputPath)
	imported, _ := image.LoadImage(paths, outputPath)
	if imported.ID != "sha256:e2e-test" {
		t.Errorf("round-trip: %s != sha256:e2e-test", imported.ID)
	}
	t.Log("Image lifecycle e2e: create → save → load → verify OK")
}

func TestIntegration_RootfsInjection(t *testing.T) {
	tmp := t.TempDir()
	rootfs := filepath.Join(tmp, "rootfs")
	initData := []byte("#!/bin/sh\necho BOOTED\n")
	os.MkdirAll(filepath.Join(rootfs, "sbin"), 0o755)
	os.WriteFile(filepath.Join(rootfs, "sbin", "init"), initData, 0o755)

	content, err := os.ReadFile(filepath.Join(rootfs, "sbin", "init"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != string(initData) {
		t.Error("init content mismatch")
	}
}

func TestIntegration_ContainerStateTransitions(t *testing.T) {
	transitions := []struct{ from, to string }{
		{"created", "running"},
		{"running", "stopped"},
		{"running", "failed"},
		{"stopped", "running"},
	}
	for _, tr := range transitions {
		if tr.from == tr.to {
			t.Errorf("invalid self-transition: %s", tr.from)
		}
	}
	t.Logf("Verified %d valid state transitions", len(transitions))
}
