# poqman — Architecture

> "podman, but for qemu" — Dockerfile-compatible container build & run powered by QEMU

## Overview

poqman builds and runs fully-emulated containers using QEMU as the isolation layer.
Each container is a QEMU microVM with a custom kernel and a root filesystem.
The Dockerfile format is supported 1:1, with a poqman-specific `KERNEL` instruction
to specify the kernel download source.

## Directory Structure (current)

```
poqman/
├── cmd/
│   ├── poqman/main.go              # Entry point, subcommand dispatch (10 commands)
│   ├── poqman-init/main.go         # PID 1 init binary for inside VMs
│   └── poqman-agent/main.go        # virtio-serial agent for exec
├── pkg/
│   ├── cli/
│   │   ├── router.go               # Subcommand router (flag.FlagSet, hand-rolled)
│   │   ├── router_test.go          # Router + dispatch tests
│   │   ├── images.go               # poqman images
│   │   ├── ps.go                   # poqman ps [-a] [-q]
│   │   ├── pull.go                 # poqman pull [--platform]
│   │   ├── run.go                  # poqman run [-d] [-it] [-p] [-v] [-m] [--cpus] [--name] [--rm]
│   │   ├── start.go                # poqman start [-a]
│   │   ├── stop.go                 # poqman stop [-t timeout]
│   │   ├── exec.go                 # poqman exec [--workdir]
│   │   ├── exec_test.go            # RegisterExec test
│   │   ├── logs.go                 # poqman logs [-f] [--tail]
│   │   └── kernel.go               # poqman kernel pull|list|rm
│   ├── image/
│   │   ├── image.go                # Image, ImageConfig, Layer, ImageIndex types
│   │   ├── name.go                 # ImageRef parser (registry/repo:tag@digest)
│   │   ├── name_test.go            # Parsing tests (simple, tag, digest, custom registry)
│   │   ├── store.go                # Local image store (CRUD + index.json)
│   │   └── store_test.go           # Store tests (Save/Get/List/Resolve/Remove)
│   ├── container/
│   │   ├── container.go            # Container, PortMapping, VolumeMount, state machine
│   │   ├── container_test.go       # GenerateID, status, defaults tests
│   │   ├── store.go                # Container state store (CRUD + state.json)
│   │   └── store_test.go           # Store tests (Create/Load/Save/List/Remove)
│   ├── registry/
│   │   ├── client.go               # OCI Distribution API HTTP client (retry, auth)
│   │   ├── auth.go                 # Docker Hub token authentication
│   │   ├── auth_test.go            # Auth header parsing tests
│   │   ├── manifest.go             # Manifest / ManifestList / OCI Config types
│   │   ├── manifest_test.go        # Manifest parsing tests
│   │   ├── pull.go                 # Pull orchestration (manifest→layers→extract→register)
│   │   ├── platform.go             # Platform matching (arch, os, variant)
│   │   └── platform_test.go        # Platform parse/match tests
│   ├── runtime/
│   │   ├── qemu.go                 # QEMU binary detection + arch mapping
│   │   ├── qemu_test.go            # Architecture/console tests
│   │   ├── args.go                 # QEMU argument + kernel cmdline builder
│   │   ├── args_test.go            # Arg builder + cmdline tests (13 subtests)
│   │   ├── process.go              # Process lifecycle (start, detach, wait, kill)
│   │   ├── qmp.go                  # QMP client over unix socket
│   │   ├── agent.go                # Host-side agent client (Execute, Signal, Ping)
│   │   └── agent_test.go           # Agent protocol + client tests (12 subtests)
│   ├── network/
│   │   ├── manager.go              # Bridge creation, TAP, iptables NAT, IPAM
│   │   └── manager_test.go         # Network state, IP allocation tests
│   ├── kernel/
│   │   ├── kernel.go               # Kernel + KernelIndex types, ParseKernelRef
│   │   ├── kernel_test.go          # KernelRef parsing + store tests
│   │   ├── store.go                # Kernel image store (CRUD + index)
│   │   ├── resolver.go             # ResolverRegistry + OCI/HTTP resolvers
│   │   ├── resolver_test.go        # Resolver registry + distro tests
│   │   ├── resolver_debian.go      # Debian .deb download + extraction
│   │   ├── resolver_alpine.go      # Alpine .apk download + extraction
│   │   └── resolver_archlinux.go   # Arch Linux kernel download + extraction
│   └── storage/
│       ├── paths.go                # XDG-compliant storage layout (30+ path helpers)
│       ├── paths_test.go           # Path resolution + all path helper tests
│       ├── layer.go                # Layer tar.gz extraction + digest verification
│       ├── layer_test.go           # Layer extraction tests
│       ├── rootfs.go               # Rootfs assembly + init injection + kernel copy
│       └── rootfs_test.go          # Rootfs assembly tests
```

## Storage Layout

```
$XDG_DATA_HOME/poqman/       (~/.local/share/poqman/)
├── images/
│   ├── index.json                    # {"nginx:latest" → "sha256:abc123", ...}
│   └── <image-id>/
│       ├── manifest.json             # OCI manifest
│       ├── config.json               # OCI image config (env, cmd, entrypoint, labels)
│       ├── kernel/                   # Bundled kernel for this image
│       │   └── bzImage
│       └── layers/
│           └── <digest>/             # Extracted layer filesystem tree
├── kernels/
│   ├── index.json                    # {"debian:6.1.0-25:amd64" → "sha256:...", ...}
│   └── <kernel-id>/
│       ├── bzImage
│       ├── config.json               # { version, distro, arch, package_url }
│       └── modules/                  # Kernel modules (optional)
├── containers/
│   └── <container-id>/
│       ├── config.json               # { image, cmd, env, kernel, network, volumes }
│       ├── state.json                # { status, pid, startedAt, finishedAt, ip }
│       ├── rootfs/                   # Merged root filesystem (layers + writable upperdir)
│       ├── kernel/                   # Symlinked/copied kernel for this container
│       │   └── bzImage
│       ├── qmp.sock                  # QMP unix socket
│       ├── monitor.sock              # QEMU monitor unix socket
│       ├── agent.sock                # poqman-agent virtio-serial socket
│       ├── console.log               # Serial console output
│       └── pidfile                   # QEMU PID file
├── networks/
│   └── state.json                    # { bridge, subnet, gateway, allocations }
└── tmp/                              # Build / pull staging
```

## Key Design Decisions

### 1. Root Filesystem: virtio-9p

Containers use the 9p filesystem protocol to mount the host-side rootfs directory
directly into the VM. No disk image creation, no loopback mounts, no root required.

```
QEMU args:
-fsdev local,id=rootfs,path=/path/to/rootfs,security_model=mapped-xattr
-device virtio-9p-pci,fsdev=rootfs,mount_tag=rootfs

Kernel cmdline:
root=rootfs rootfstype=9p rootflags=trans=virtio,version=9p2000.L rw
```

The custom kernel must include `CONFIG_9P_FS=y` and `CONFIG_9P_FS_POSIX_ACL=y`.

Volume mounts (`-v`) create additional 9p fsdev entries, mounted by poqman-init
inside the VM using `poqman.volume.N.*` kernel cmdline parameters.

### 2. Kernel: Distribution Package Download

The `KERNEL "distro:version"` instruction downloads the kernel package from the
distribution's repository and extracts the bootable kernel image.

Kernel references support automatic architecture detection:
- `debian:6.1.0-25-amd64` → full version (distro + arch)
- `debian:6.1.0-25:amd64` → explicit arch
- `alpine:3.21:lts:6.6.52-0-lts` → release:flavor:version
- `archlinux:6.10.10:arch1-1` → version:pkg-version

If the last colon-separated segment matches a known architecture
(amd64, arm64, arm, armhf, i386, riscv64, ppc64le, s390x, x86_64, aarch64),
it is parsed as an explicit architecture override. Otherwise the host
architecture is used.

| Distro | Package format | Extraction |
|--------|---------------|------------|
| debian | `.deb` (ar archive) | `ar x` → `tar xzf` data.tar.gz |
| alpine | `.apk` (tar.gz) | `tar xzf` |
| archlinux | `.pkg.tar.zst` | `zstdcat | tar xf` |
| oci | OCI image | Full registry pull + `/boot/vmlinuz` search |
| http/https | Direct URL | Auto-detects format from extension |

**Important LIMITATION (MVP):** Distribution resolvers require full package
version strings. Example: `debian:6.1.0-25-amd64:6.1.106-3`. The resolvers
will be enhanced in a future phase with package metadata API lookups.

### 3. PID 1 Inside VM: poqman-init

A static Go binary (`CGO_ENABLED=0`, `GOOS=linux`) injected as `/sbin/init` into
every container at run time.

Responsibilities:
1. Mount proc, sys, devtmpfs, devpts
2. Parse `poqman.*` parameters from `/proc/cmdline`
3. Set hostname from `poqman.hostname=`
4. Configure network: `ip addr`, `ip route`, from `poqman.ip=` / `poqman.gateway=`
5. Mount volumes from `poqman.volume.N.*` parameters
6. Execute CMD from `poqman.cmd=` via `/bin/sh -c`
7. Handle signals: SIGTERM/SIGINT → forward to child (5s grace → SIGKILL)
8. On child exit or SIGCHLD: sync + `reboot(RB_POWER_OFF)` → QEMU exits

### 4. Exec: poqman-agent via virtio-serial

A background agent running inside the VM, communicating over a virtio-serial port.

**Host QEMU args:**
```
-chardev socket,id=agent0,path=<agent.sock>,server=on,wait=off
-device virtio-serial-pci
-device virtserialport,chardev=agent0,name=poqman.agent
```

**Protocol:** JSON lines over the unix socket.

```
Request:  {"id":1, "command":"execute", "args":["cat","/etc/hosts"],
           "env":["PATH=/usr/bin"], "cwd":"/"}
Response: {"id":1, "exit":0, "stdout":"127.0.0.1 localhost\n", "stderr":""}

Request:  {"id":2, "command":"ping"}
Response: {"id":2, "ok":true}

Request:  {"id":3, "command":"signal", "signal":"SIGTERM"}
Response: {"id":3, "ok":true}
```

**Host-side client:** `runtime.AgentClient` with `Execute()`, `Signal()`, `Ping()` methods,
connecting over the unix socket at `containers/<id>/agent.sock`.

### 5. Networking: Bridge + TAP + iptables NAT

On first run, poqman creates a persistent bridge:

```
ip link add poqman0 type bridge
ip addr add 10.88.0.1/16 dev poqman0
ip link set poqman0 up
iptables -t nat -A POSTROUTING -s 10.88.0.0/16 -j MASQUERADE
iptables -A FORWARD -i poqman0 -j ACCEPT
iptables -A FORWARD -o poqman0 -j ACCEPT
```

Per container: create `tap-<shortid>`, enslave in bridge, allocate IP from
10.88.0.0/16 via simple sequential IPAM. Port forwarding via `-p` uses
iptables DNAT rules. All managed by `network.Manager`.

### 6. Architecture Mapping

| Go GOARCH | OCI Platform | QEMU Binary | Machine | Console |
|-----------|-------------|-------------|---------|---------|
| amd64 | linux/amd64 | qemu-system-x86_64 | q35 | ttyS0 |
| arm64 | linux/arm64 | qemu-system-aarch64 | virt | ttyAMA0 |
| arm | linux/arm/v7 | qemu-system-arm | virt | ttyAMA0 |
| riscv64 | linux/riscv64 | qemu-system-riscv64 | virt | ttyAMA0 |
| ppc64le | linux/ppc64le | qemu-system-ppc64 | pseries | ttyS0 |

### 7. Image Pull Flow

```
1. Parse image ref (name.go) → registry, repo, tag
2. GET /v2/<repo>/manifests/<tag> (multi Accept header)
3. If manifest list → MatchPlatform() → resolve per-arch manifest
4. GET /v2/<repo>/blobs/<config-digest> → parse OCI image config
5. Compute image ID = sha256(config-blob)
6. For each layer: GET blob → verify digest → extract tar.gz → store in layers/<digest>/
7. Save image config + update index.json
```

### 8. Dockerfile Build Process (Phase 7 — planned)

1. **Parse** Dockerfile → AST (instructions)
2. **FROM**: Pull base image layers, extract to working rootfs
3. **KERNEL**: Download/extract kernel package → store in image
4. **For each RUN**: Boot QEMU with build rootfs via 9p, run command, capture diff → layer
5. **For each COPY/ADD**: Copy files into rootfs, create layer
6. **For ENV, CMD, etc.**: Update image config in memory
7. **Commit**: Store image config, layers, kernel → image store

### 9. Dependencies

| Dependency | Source | Usage |
|---|---|---|
| net/http | stdlib | Registry client |
| encoding/json | stdlib | All config/state files |
| archive/tar | stdlib | Layer & package extraction |
| compress/gzip | stdlib | Gzip decompression |
| os/exec | stdlib | QEMU, ip, bridge, iptables, ar, xzcat, zstdcat |
| flag | stdlib | CLI argument parsing |
| syscall | stdlib | poqman-init: mount, sethostname, reboot, signals |
| embed | stdlib | Embed poqman-init binary |
| crypto/sha256 | stdlib | Digest verification |
| golang.org/x/sys/unix | x | Signal handling in poqman-init, process group kill |
| golang.org/x/term | x | Terminal raw mode for -it interactive attach |

**Zero third-party dependencies.** Only stdlib + golang.org/x where necessary.

### 10. Container State Machine

```
created ──► running ──► stopped
                │            │
                ▼            ▼
             failed       removed
```

### 11. Test Coverage (123 tests, all passing)

| Package | Coverage | Tests |
|---------|----------|-------|
| pkg/container | 82.9% | 11 |
| pkg/image | 80.9% | 15 |
| pkg/storage | 71.1% | 12 |
| pkg/runtime | 45.0% | 30 |
| pkg/kernel | 42.1% | 17 |
| pkg/network | 30.1% | 9 |
| pkg/registry | 20.8% | 20 |
| pkg/cli | 7.4% | 9 |

Lower coverage in runtime/kernel/network/registry is expected for packages
with heavy I/O, system calls, and external HTTP dependencies.
Core logic paths are well-covered.
