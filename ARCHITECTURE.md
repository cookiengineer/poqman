# poqman — Architecture

> "podman, but for qemu" — Dockerfile-compatible container build & run powered by QEMU

## Overview

poqman builds and runs fully-emulated containers using QEMU as the isolation layer.
Each container is a QEMU microVM with a custom kernel and a root filesystem.
15 CLI commands. 269 tests. Zero third-party dependencies.

## Directory Structure

```
poqman/
├── cmd/
│   ├── poqman/main.go              # Entry point, 15 subcommands
│   ├── poqman-init/main.go         # PID 1 init (Go binary, CGO_ENABLED=0, GOOS=linux)
│   └── poqman-agent/main.go        # virtio-serial exec agent
├── pkg/
│   ├── cli/
│   │   ├── router.go / router_test.go      # Flag-based subcommand router
│   │   ├── embed.go / embed_test.go        # go:embed init/agent + shell fallback
│   │   ├── images.go, ps.go               # images, ps
│   │   ├── pull.go                        # pull [--platform]
│   │   ├── run.go                         # run [-d] [-it] [-p] [-v] [-m] [--cpus] [--name] [--rm]
│   │   ├── start.go                       # start [-a]
│   │   ├── stop.go / stop_test.go         # stop [-t] + forceKill with port/vol cleanup
│   │   ├── exec.go / exec_test.go         # exec [--workdir] (agent retry, 15s timeout)
│   │   ├── logs.go                        # logs [-f] [--tail]
│   │   ├── kernel.go                      # kernel pull|list|rm
│   │   ├── rm.go / rm_test.go             # rm [-f] + rmi [-f]
│   │   ├── inspect.go / inspect_test.go   # inspect (JSON output)
│   │   ├── build.go / build_test.go       # build -t <tag> [-f <Dockerfile>] [--platform]
│   │   └── saveload.go                    # save [-o <file>] + load -i <file>
│   ├── image/
│   │   ├── image.go, name.go, name_test.go   # Image/ImageIndex types, ImageRef parser
│   │   ├── store.go, store_test.go           # CRUD + thread-safe RWMutex index
│   │   └── saveload.go                       # SaveImage / LoadImage (tar.gz export/import)
│   ├── container/
│   │   ├── container.go, container_test.go   # Container types, state machine, PortMapping, VolumeMount
│   │   ├── store.go, store_test.go           # Container state CRUD + state.json
│   │   └── health.go                         # HealthConfig, HealthStatus, HealthState types
│   ├── registry/
│   │   ├── client.go, auth.go, auth_test.go        # OCI Distribution API, Docker Hub auth
│   │   ├── manifest.go, manifest_test.go           # Manifest/ManifestList/OCIConfig types
│   │   ├── pull.go                                # Pull orchestration
│   │   └── platform.go, platform_test.go           # Platform matching (arch, os, variant)
│   ├── runtime/
│   │   ├── qemu.go, qemu_test.go         # QEMU detection, architecture mapping
│   │   ├── args.go, args_test.go         # QEMU argument + kernel cmdline builder
│   │   ├── process.go                    # Process lifecycle (start, detach, wait, kill)
│   │   ├── qmp.go                        # QMP client over unix socket
│   │   ├── agent.go, agent_test.go       # Host-side client (Execute, Signal, Ping)
│   │   ├── tty.go                        # golang.org/x/term raw mode for -it
│   │   └── cgroup.go                     # ApplyCGroupLimits / RemoveCGroup (memory, CPU, pids)
│   ├── network/
│   │   ├── manager.go, manager_test.go   # Bridge, TAP, iptables NAT, IPAM
│   │   └── ipv6.go                       # SetupIPv6, AllocateIPv6, EnableDHCP, StartDHCPServer
│   ├── kernel/
│   │   ├── kernel.go, kernel_test.go     # Kernel/KernelIndex types, ParseKernelRef
│   │   ├── store.go                      # Kernel image store
│   │   ├── resolver.go, resolver_test.go # ResolverRegistry + OCI/HTTP resolvers
│   │   ├── resolver_debian.go            # .deb download + ar extraction
│   │   ├── resolver_alpine.go            # .apk download + tar.gz extraction
│   │   ├── resolver_archlinux.go         # .pkg.tar.zst download + zstd extraction
│   │   ├── resolver_api.go               # Auto-resolution via distro package APIs
│   │   ├── resolver_api_test.go          # Integration tests (real API calls)
│   │   └── puller.go                     # Kernel pull orchestrator
│   ├── dockerfile/
│   │   ├── ast.go                        # 17 AST instruction types (incl. HEALTHCHECK)
│   │   ├── parser.go, parser_test.go     # Scanner + recursive descent parser
│   │   ├── builder.go, builder_test.go   # Build engine with QEMU-based RUN
│   │   ├── dockerignore.go               # .dockerignore pattern engine
│   │   ├── dockerignore_test.go          # Ignore tests (12 cases)
│   │   ├── difflayer.go                  # Diff-based layer tar.gz creation
│   │   └── difflayer_test.go             # Diff layer tests (6 cases)
│   ├── storage/
│   │   ├── paths.go, paths_test.go       # XDG layout, 30+ path helpers
│   │   ├── layer.go, layer_test.go       # tar.gz extraction + digest verification
│   │   └── rootfs.go, rootfs_test.go     # Rootfs assembly + init/agent injection
│   └── lifecycle/
│       ├── lifecycle_test.go             # E2E container+image lifecycle (17 tests)
│       └── integration_test.go           # System integration tests (14 tests, QEMU, save/load)
```

## Storage Layout

```
$XDG_DATA_HOME/poqman/       (~/.local/share/poqman/)
├── images/
│   ├── index.json                    # {"nginx:latest" → "sha256:abc123", ...}
│   └── <image-id>/
│       ├── manifest.json
│       ├── config.json               # OCI image config
│       ├── kernel/bzImage            # Bundled kernel
│       └── layers/<digest>/          # Extracted layer filesystem tree
├── kernels/
│   ├── index.json
│   └── <kernel-id>/
│       ├── bzImage
│       └── config.json
├── containers/
│   └── <container-id>/
│       ├── config.json / state.json
│       ├── rootfs/                   # Merged rootfs (layers + writable diff)
│       ├── kernel/bzImage
│       ├── qmp.sock / monitor.sock / agent.sock
│       ├── console.log / pidfile
├── networks/
│   └── state.json                    # Bridge, subnet, gateway, IP allocations
└── tmp/                              # Build / pull staging
```

## Key Design Decisions

### 1. Root Filesystem: virtio-9p

Containers mount the host-side rootfs directory directly into the VM via 9p.
No disk image creation, no loopback mounts, no root required.

```
QEMU args:
-fsdev local,id=rootfs,path=<rootfs>,security_model=mapped-xattr
-device virtio-9p-pci,fsdev=rootfs,mount_tag=rootfs

Kernel cmdline:
root=rootfs rootfstype=9p rootflags=trans=virtio,version=9p2000.L rw
```

### 2. Kernel: Distribution Package Download

`KERNEL "distro:version"` downloads and extracts kernel packages.
Auto-resolution via distro package APIs when full version is omitted.

| Distro | Package | Auto-Resolution API |
|--------|---------|---------------------|
| debian | `.deb` (ar) | `api.ftp-master.debian.org/madison` |
| alpine | `.apk` (tar.gz) | `pkgs.alpinelinux.org` |
| archlinux | `.pkg.tar.zst` | `archive.archlinux.org` |
| oci | OCI image | Full registry pull |
| http/https | Direct URL | Format auto-detected |

### 3. PID 1 Inside VM: poqman-init

Two modes: embedded Go binary (`make embed`) or POSIX shell script fallback.
Mounts filesystems, configures network, execs CMD from kernel cmdline.

### 4. Exec: poqman-agent via virtio-serial

JSON-lines protocol over virtio-serial. Host connects via unix socket.
Retries connection up to 15 seconds for VM boot completion.

### 5. Networking: Bridge + TAP + iptables NAT

Persistent bridge `poqman0` at 10.88.0.1/16 (IPv4) + `fd00:dead:beef::/64` (IPv6).
Dual-stack with per-container TAP devices and sequential IPAM.
Port forwarding via iptables DNAT, cleaned up on stop. DHCP via dnsmasq.

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
Parse ref → GET manifest (multi Accept) → resolve per-arch if manifest list
→ GET config blob → compute image ID = sha256(config)
→ for each layer: GET blob → verify digest → extract tar.gz → store
→ save image config + update index.json (thread-safe RWMutex)
```

### 8. Dockerfile Build Process (17 instruction types)

1. **Parse** → scanner merges line continuations, parser produces AST
2. **Load .dockerignore** → wildcard/directory/negate patterns for COPY/ADD filtering
3. **FROM**: Pull + extract base image layers into build rootfs
4. **KERNEL**: Pull + extract kernel (auto-resolves versions via distro APIs)
5. **RUN**: With KERNEL + QEMU: snapshot → boot VM with 9p rootfs → execute → compute diff → create tar.gz layer. Falls back to recording-only otherwise.
6. **COPY/ADD**: Copy from build context, respecting .dockerignore
7. **HEALTHCHECK**: Parse --interval/--timeout/--retries/--start-period + CMD
8. **Commit**: Store image config, layers, kernel → image store

### 9. Container State Machine

```
created ──► running ──► stopped
                │            │
                ▼            ▼
             failed       removed
```

Health status: `starting` → `healthy` / `unhealthy` (via HEALTHCHECK command execution)

### 10. Resource Limits

Cgroup-based limits via `runtime.ApplyCGroupLimits()`:
- `memory.max` — container memory cap
- `cpu.weight` — CPU shares for fair scheduling
- `pids.max` — process count limit
- QEMU `-m` for memory, `-smp` for CPU cores

### 11. Image Save/Load

`poqman save` exports an image as a gzipped tar archive containing
`manifest.json` + layer directories + kernel. `poqman load` imports
from such archives, registering the image in the local store.

### 12. Dependencies

| Dependency | Source | Usage |
|---|---|---|
| net/http | stdlib | Registry client, distro API queries |
| encoding/json | stdlib | All config/state files |
| archive/tar | stdlib | Layer & package extraction, save/load |
| compress/gzip | stdlib | Gzip decompression |
| os/exec | stdlib | QEMU, ip, bridge, iptables, ar, xzcat, zstdcat |
| flag | stdlib | CLI argument parsing |
| syscall | stdlib | poqman-init: mount, sethostname, reboot, signals |
| embed | stdlib | Embed poqman-init + poqman-agent binaries |
| crypto/sha256 | stdlib | Digest verification |
| sync | stdlib | Thread-safe image index access |
| golang.org/x/sys/unix | x | Signal handling, process group kill |
| golang.org/x/term | x | Terminal raw mode for -it interactive attach |

**Zero third-party dependencies.** stdlib + golang.org/x only.

### 13. Test Coverage (259 tests, 0 skips in unit tests)

**All kernel API tests use `t.Fatalf`** — no silent skips when network resources
change. All integration tests (QEMU VM boot, Dockerfile build) run on hosts
with QEMU and skip only for missing QEMU binary.

| Package | Coverage | Tests |
|---------|----------|-------|
| pkg/container | 82.9% | 11 |
| pkg/storage | 68.6% | 12 |
| pkg/dockerfile | 54.1% | 92 |
| pkg/image | 52.9% | 18 |
| pkg/kernel | 43.9% | 27 |
| pkg/runtime | 40.9% | 30 |
| pkg/network | 23.9% | 9 |
| pkg/registry | 20.8% | 20 |
| pkg/cli | 14.5% | 46 |
| pkg/lifecycle | e2e | 13 |

### 14. Supported Linux Distributions for Kernels

| Distro | Status | Auto-Resolution | Package Pool |
|--------|--------|-----------------|--------------|
| Debian | ✅ | `api.ftp-master.debian.org/madison` | `linux-signed-amd64` |
| Alpine | ✅ | `pkgs.alpinelinux.org` (HTML parser) | `dl-cdn.alpinelinux.org` |
| Arch Linux | ✅ | `archive.archlinux.org` (HTML scraper) | `archive.archlinux.org` |
| OCI | ✅ | OCI registry pull | `docker.io` |
| HTTP/HTTPS | ✅ | Direct URL download | Any |
| Ubuntu | ✅ | `archive.ubuntu.com` (pool scraper) + `zstdcat` extraction | `archive.ubuntu.com/ubuntu/pool/main/l/linux-signed/` |

Lower coverage in runtime/kernel/network/registry is expected for packages
with heavy I/O, system calls, and external HTTP/network dependencies.
Core logic paths well-covered. End-to-end + integration tests in `pkg/lifecycle/`.
