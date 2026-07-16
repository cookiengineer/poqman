package cli

import (
	"embed"
	"fmt"
	"runtime"
)

//go:embed bin/*
var embeddedBinaries embed.FS

func getEmbeddedBinary(name string) ([]byte, error) {
	data, err := embeddedBinaries.ReadFile("bin/" + name)
	if err != nil {
		return nil, fmt.Errorf("embedded binary %q not found (run 'make embed' first): %w", name, err)
	}
	return data, nil
}

func InitBinary(goarch string) []byte {
	arch := goarch
	if arch == "" {
		arch = runtime.GOARCH
	}

	names := []string{
		"poqman-init-" + arch,
		"poqman-init",
	}

	for _, name := range names {
		if data, err := getEmbeddedBinary(name); err == nil {
			return data
		}
	}

	return defaultInitBinary()
}

func AgentBinary(goarch string) []byte {
	arch := goarch
	if arch == "" {
		arch = runtime.GOARCH
	}

	names := []string{
		"poqman-agent-" + arch,
		"poqman-agent",
	}

	for _, name := range names {
		if data, err := getEmbeddedBinary(name); err == nil {
			return data
		}
	}

	return defaultAgentBinary()
}

func defaultInitBinary() []byte {
	return defaultShellScript
}

func defaultAgentBinary() []byte {
	return []byte{}
}

var defaultShellScript = []byte(`#!/bin/sh
mount -t proc proc /proc
mount -t sysfs sys /sys
mount -t devtmpfs dev /dev
mkdir -p /dev/pts
mount -t devpts devpts /dev/pts

hostname=$(cat /proc/cmdline | tr ' ' '\n' | grep '^poqman.hostname=' | sed 's/poqman.hostname=//')
if [ -n "$hostname" ]; then
    hostname "$hostname"
fi

ip=$(cat /proc/cmdline | tr ' ' '\n' | grep '^poqman.ip=' | sed 's/poqman.ip=//')
gateway=$(cat /proc/cmdline | tr ' ' '\n' | grep '^poqman.gateway=' | sed 's/poqman.gateway=//')

if [ -n "$ip" ]; then
    iface=$(ip link show | grep -o 'eth[0-9]*' | head -1)
    [ -z "$iface" ] && iface=$(ip link show | grep -o 'enp[0-9a-z]*' | head -1)
    [ -z "$iface" ] && iface=$(ls /sys/class/net | grep -v lo | head -1)
    if [ -n "$iface" ]; then
        ip addr add "$ip" dev "$iface"
        ip link set "$iface" up
        [ -n "$gateway" ] && ip route add default via "$gateway"
    fi
fi

cmd=$(cat /proc/cmdline | tr ' ' '\n' | grep '^poqman.cmd=' | sed 's/poqman.cmd=//')
if [ -z "$cmd" ]; then
    cmd="/bin/sh"
fi

exec /bin/sh -c "$cmd"
`)
