package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func main() {
	if os.Getpid() != 1 {
		fmt.Fprintln(os.Stderr, "poqman-init must be run as PID 1")
		os.Exit(1)
	}

	if isInitrdMode() {
		runInitrd()
	}

	mountProc()
	mountSys()
	mountDev()
	mountDevPts()

	params := parseCmdline()

	if hostname, ok := params["poqman.hostname"]; ok {
		syscall.Sethostname([]byte(hostname))
	}

	setupNetwork(params)

	mountVolumes(params)

	command := params["poqman.cmd"]
	if command == "" {
		command = "/bin/sh"
	}

	shell := "/bin/sh"
	if _, err := os.Stat(shell); os.IsNotExist(err) {
		shell = "/bin/bash"
	}

	fmt.Printf("[poqman-init] starting: %s\n", command)
	cmd := exec.Command(shell, "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[poqman-init] failed to start command: %v\n", err)
		shutdown(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGCHLD)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGCHLD:
				shutdown(0)
			case syscall.SIGTERM, syscall.SIGINT:
				if cmd.Process != nil {
					cmd.Process.Signal(sig)
				}
				time.Sleep(5 * time.Second)
				if cmd.Process != nil {
					cmd.Process.Signal(syscall.SIGKILL)
				}
				shutdown(0)
			}
		}
	}()

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	shutdown(exitCode)
}

func isInitrdMode() bool {
	return os.Args[0] == "/init"
}

func runInitrd() {
	os.MkdirAll("/proc", 0o755)
	os.MkdirAll("/sys", 0o755)
	os.MkdirAll("/dev", 0o755)

	syscall.Mount("proc", "/proc", "proc", 0, "")
	syscall.Mount("sysfs", "/sys", "sysfs", 0, "")
	syscall.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")

	loadNinePModules()

	os.MkdirAll("/newroot", 0o755)
	if err := syscall.Mount("rootfs", "/newroot", "9p", 0, "trans=virtio,version=9p2000.L"); err != nil {
		fmt.Fprintf(os.Stderr, "[poqman-init] initrd: mount rootfs failed: %v\n", err)
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	}

	ensureInit()

	copyBusybox()

	realInit := readInitParam()
	if realInit == "" {
		realInit = "/sbin/init"
	}

	syscall.Unmount("/proc", 0)
	syscall.Unmount("/sys", 0)

	syscall.Chdir("/newroot")
	syscall.Chroot(".")
	syscall.Chdir("/")

	if err := syscall.Exec(realInit, []string{realInit}, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "[poqman-init] initrd: exec %s failed: %v\n", realInit, err)
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	}
}

func readInitParam() string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	params := strings.Fields(string(data))
	for _, param := range params {
		if strings.HasPrefix(param, "init=") {
			val := strings.TrimPrefix(param, "init=")
			return val
		}
	}
	return ""
}

func ensureInit() {
	initPath := "/newroot/sbin/init"
	if _, err := os.Stat(initPath); err == nil {
		return
	}
	selfData, err := os.ReadFile("/init")
	if err != nil {
		return
	}
	os.MkdirAll("/newroot/sbin", 0o755)
	os.WriteFile(initPath, selfData, 0o755)
}

func copyBusybox() {
	bbPath := "/bin/busybox"
	if _, err := os.Stat(bbPath); os.IsNotExist(err) {
		return
	}
	os.MkdirAll("/newroot/bin", 0o755)
	copyFile(bbPath, "/newroot/bin/busybox")
	for _, app := range []string{"sh", "ip", "udhcpc", "ifconfig", "route"} {
		src := "/bin/" + app
		if _, err := os.Stat(src); err == nil {
			copyFile(src, "/newroot/bin/"+app)
		}
	}
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.WriteFile(dst, data, 0o755)
}

func loadNinePModules() {
	entries, err := os.ReadDir("/lib/modules")
	if err != nil {
		return
	}

	loaded := make(map[string]bool)
	for attempt := 0; attempt < 20; attempt++ {
		anyLoaded := false
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".ko") {
				continue
			}
			modPath := "/lib/modules/" + entry.Name()
			if loaded[modPath] {
				continue
			}
			fd, err := syscall.Open(modPath, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
			if err != nil {
				continue
			}
			if err := unix.FinitModule(fd, "", 0); err == nil {
				loaded[modPath] = true
				anyLoaded = true
			}
			syscall.Close(fd)
		}
		if !anyLoaded {
			break
		}
	}
}

func mountProc() {
	os.MkdirAll("/proc", 0o755)
	syscall.Mount("proc", "/proc", "proc", 0, "")
}

func mountSys() {
	os.MkdirAll("/sys", 0o755)
	syscall.Mount("sysfs", "/sys", "sysfs", 0, "")
}

func mountDev() {
	os.MkdirAll("/dev", 0o755)
	syscall.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")
}

func mountDevPts() {
	os.MkdirAll("/dev/pts", 0o755)
	syscall.Mount("devpts", "/dev/pts", "devpts", 0, "")
}

func parseCmdline() map[string]string {
	data, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return map[string]string{}
	}

	params := make(map[string]string)
	for _, part := range strings.Fields(string(data)) {
		if !strings.HasPrefix(part, "poqman.") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			val := unescapeArg(kv[1])
			params[kv[0]] = val
		} else {
			params[kv[0]] = "1"
		}
	}
	return params
}

func setupNetwork(params map[string]string) {
	ip := params["poqman.ip"]
	if ip == "" {
		return
	}

	gateway := params["poqman.gateway"]
	dns := params["poqman.dns"]

	iface := findNetworkInterface()
	if iface == "" {
		fmt.Fprintln(os.Stderr, "[poqman-init] no network interface found")
		return
	}

	cmd := exec.Command("ip", "addr", "add", ip, "dev", iface)
	cmd.Run()

	cmd = exec.Command("ip", "link", "set", iface, "up")
	cmd.Run()

	if gateway != "" {
		cmd = exec.Command("ip", "route", "add", "default", "via", gateway)
		cmd.Run()
	}

	if dns != "" {
		os.WriteFile("/etc/resolv.conf", []byte("nameserver "+dns+"\n"), 0o644)
	}
}

func findNetworkInterface() string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "lo" {
			return name
		}
	}
	return ""
}

func mountVolumes(params map[string]string) {
	for i := 0; ; i++ {
		sourceKey := fmt.Sprintf("poqman.volume.%d.source", i)
		targetKey := fmt.Sprintf("poqman.volume.%d.target", i)
		readonlyKey := fmt.Sprintf("poqman.volume.%d.readonly", i)

		source := params[sourceKey]
		target := params[targetKey]
		if source == "" || target == "" {
			break
		}

		os.MkdirAll(target, 0o755)

		mountOpts := "trans=virtio,version=9p2000.L"
		if params[readonlyKey] == "true" {
			mountOpts += ",ro"
		} else {
			mountOpts += ",rw"
		}

		mountTag := fmt.Sprintf("volume%d", i)
		if err := syscall.Mount(mountTag, target, "9p", 0, mountOpts); err != nil {
			fmt.Fprintf(os.Stderr, "[poqman-init] mount volume %s: %v\n", mountTag, err)
		}
	}
}

func unescapeArg(arg string) string {
	if len(arg) >= 2 && arg[0] == '"' && arg[len(arg)-1] == '"' {
		arg = arg[1 : len(arg)-1]
	}
	return strings.ReplaceAll(arg, "\\\"", "\"")
}

func shutdown(code int) {
	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}
