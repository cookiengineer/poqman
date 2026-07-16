# poqman — ROADMAP

## Phase 1: Foundation ✅
**Goal:** Bootable skeleton — types, stores, CLI framework, `poqman images`, `poqman ps`

- [x] `pkg/storage/paths.go` — XDG-compliant storage layout, mkdir helpers
- [x] `pkg/image/image.go` — Image, ImageConfig, Layer types
- [x] `pkg/image/name.go` — Image name parser (registry/repo:tag@digest)
- [x] `pkg/image/store.go` — Local image store (index.json read/write, layer listing)
- [x] `pkg/container/container.go` — Container struct (ID, image, state, PID)
- [x] `pkg/container/store.go` — Container state store (JSON per container, list/mark)
- [x] `pkg/cli/router.go` — Subcommand router (flag.FlagSet per command)
- [x] `cmd/poqman/main.go` — Entry point, dispatch to router
- [x] `pkg/cli/images.go` — `poqman images` — list local images
- [x] `pkg/cli/ps.go` — `poqman ps` / `poqman ps -a` — list containers
- [x] Tests: image name parsing, image store CRUD, container store CRUD, CLI router, paths

## Phase 2: Registry & Pull ✅
**Goal:** Pull OCI images from docker.io, extract layers, store locally

- [x] `pkg/registry/client.go` — HTTP client with retry, auth header injection
- [x] `pkg/registry/auth.go` — Docker Hub token authentication
- [x] `pkg/registry/manifest.go` — Manifest / ManifestList / OCI Config types
- [x] `pkg/registry/platform.go` — Platform matching (arch, os, variant)
- [x] `pkg/registry/pull.go` — Pull orchestration: manifest → layers → extract → register
- [x] `pkg/storage/layer.go` — Layer download, digest verification, tar.gz extraction
- [x] `pkg/cli/pull.go` — `poqman pull <image>` / `poqman pull --platform <p> <image>`
- [x] Verified: pulled `alpine:latest` from docker.io, multi-platform pull works
- [x] Tests: auth parsing, manifest parsing, platform matching

## Phase 3: Kernel Store ✅
**Goal:** Download & cache kernel packages from distributions

- [x] `pkg/kernel/kernel.go` — KernelImage type, KernelIndex, ParseKernelRef
- [x] `pkg/kernel/store.go` — Kernel image store (index, download cache, extract)
- [x] `pkg/kernel/resolver.go` — ResolverRegistry + OCI/HTTP resolvers
- [x] `pkg/kernel/resolver_debian.go` — Download .deb, extract with ar + tar
- [x] `pkg/kernel/resolver_alpine.go` — Download .apk, extract tar.gz
- [x] `pkg/kernel/resolver_archlinux.go` — Download .pkg.tar.zst, extract
- [x] `pkg/cli/kernel.go` — `poqman kernel pull/list/rm`
- [x] Tests: ParseKernelRef, KernelIndex, Store CRUD, ResolverRegistry, distro resolvers

## Phase 4: QEMU Runtime & Networking ✅
**Goal:** Run containers — QEMU launch, lifecycle, networking

- [x] `pkg/runtime/qemu.go` — QEMU binary detection, version check, arch mapping
- [x] `pkg/runtime/args.go` — QEMU argument builder (kernel, 9p, net, qmp, agent, console)
- [x] `pkg/runtime/process.go` — Process lifecycle (start, wait, kill, pidfile)
- [x] `pkg/runtime/qmp.go` — QMP client over unix socket (system_powerdown, query-status)
- [x] `pkg/network/manager.go` — Bridge creation, TAP devices, iptables NAT, IPAM
- [x] `cmd/poqman-init/main.go` — PID 1 init binary (CGO_ENABLED=0, GOOS=linux)
- [x] `pkg/storage/rootfs.go` — Rootfs assembly from image layers (file copy)
- [x] `pkg/cli/run.go` — `poqman run [opts] <image> [cmd]`
- [x] `pkg/cli/start.go` — `poqman start <container>`
- [x] `pkg/cli/stop.go` — `poqman stop <container>`
- [x] `pkg/cli/logs.go` — `poqman logs <container>`
- [x] Tests: QEMU args, cmdline builder, MAC gen, arch mapping, network state, IP allocation

## Phase 5: Agent & Exec ✅
**Goal:** `poqman exec` to run commands inside running containers

- [x] `cmd/poqman-agent/main.go` — virtio-serial agent (CGO_ENABLED=0, GOOS=linux)
- [x] `pkg/runtime/agent.go` — Host-side AgentClient (Execute, Signal, Ping)
- [x] `pkg/cli/exec.go` — `poqman exec <container> <cmd>`
- [x] Tests: protocol marshal, agent client (pipe + unix socket), error handling

## Phase 6: Lifecycle Management ✅
**Goal:** Container/image removal, inspection

- [x] `pkg/cli/rm.go` — `poqman rm [-f] <container>` + `poqman rmi [-f] <image>`
- [x] `pkg/cli/inspect.go` — `poqman inspect <container|image>` (JSON output)
- [x] Tests: forceKill, image-in-use protection, inspect for all container states,
  inspect JSON round-trip, full image config inspection

## Phase 7: Build Engine ✅
**Goal:** `poqman build` from Dockerfile

- [x] `pkg/dockerfile/ast.go` — 16 AST instruction types
- [x] `pkg/dockerfile/parser.go` — Scanner + recursive descent parser
- [x] `pkg/dockerfile/builder.go` — Build engine (FROM, KERNEL, COPY, ADD, ENV, CMD, ...)
- [x] `pkg/cli/build.go` — `poqman build -t <tag> [-f <Dockerfile>] [--platform]`
- [x] Tests: 66 tests covering scanner, parser (44), builder (22)
- [x] Verified: real Dockerfile build (FROM debian:bookworm-slim, ENV, WORKDIR, COPY, CMD)

## Phase 8: Remaining Work

### High Priority
- [ ] **QEMU-based RUN execution** — Currently RUN instructions are recorded but not
  executed during build. Need to boot QEMU with build rootfs, run command,
  capture filesystem diff, and commit as a new layer.
- [ ] **Layer diffing for builds** — Before/after file manifest comparison to create
  minimal layer tarballs instead of full rootfs copies.
- [ ] **poqman-init embedded into poqman binary** — Currently `poqman-init` and
  `poqman-agent` are separate binaries. Should be embedded via `go:embed` and
  injected automatically.

### Medium Priority
- [ ] **Distribution kernel auto-resolution** — Package metadata API lookups so
  `KERNEL "debian:6.1.0-25-amd64"` works without manual pkg-version suffix.
- [ ] **System test suite** — End-to-end tests that launch real QEMU VMs.
- [ ] **Container port cleanup on stop/rm** — iptables DNAT rule removal.
- [ ] **agent socket readiness wait** — Retry logic in `poqman exec` until the
  virtio-serial socket appears after container start.
- [ ] **Thread-safe image pull** — Locking around index.json and layer writes.

### Low Priority / Nice to Have
- [ ] `poqman push` — Push images to OCI registries
- [ ] `poqman compose` — docker-compose.yml support
- [ ] Multi-stage builds (FROM ... AS + COPY --from=stage)
- [ ] Layer caching for builds
- [ ] Health checks
- [ ] Resource limits (cgroups for QEMU)
- [ ] IPv6 networking support
- [ ] DHCP-based IP assignment for containers
- [ ] `podman save` / `podman load` equivalent (image export/import)

---

## Known Limitations & Workarounds

### Distribution Kernel Resolvers
- **Debian resolver** requires full package version (e.g., `debian:6.1.0-25-amd64:6.1.106-3`).
  Without it, a helpful error message with a packages.debian.org search URL is shown.
- **Alpine resolver** requires `release:flavor:version` format (e.g., `alpine:3.21:lts:6.6.52-0-lts`).
- **Arch Linux resolver** requires kernel version + pkg suffix (e.g., `archlinux:6.10.10:arch1-1`).
- **Workaround:** Use OCI kernel images (`KERNEL "docker.io/poqman/kernel-*"`) which work
  with the fully-implemented OCI pull infrastructure.

### RUN Instruction During Build
- `RUN` is parsed correctly but does **not** execute via QEMU during build.
- The command string is recorded in image history and will execute at container startup.
- This means images built with `RUN apt-get install ...` will NOT have those packages
  installed until the container first boots. Use a pre-built base image instead.
- **Workaround:** Build base images separately with pre-installed packages, then
  `FROM` that image in your Dockerfile.

### Networking
- Bridge interfaces require `iproute2` and `iptables` installed on the host.
- IP forwarding must be permitted (`net.ipv4.ip_forward=1`).
- No IPv6 support.
- No DHCP server for containers — static IP via kernel cmdline only.
- Port forwarding DNAT rules are not cleaned up on container removal.

### Agent Socket Timing
- The `agent.sock` may not be immediately available after container start
  (VM needs time to boot and bind the socket).
- `poqman exec` has no retry logic; if the agent isn't ready, it fails immediately.

### Rootfs Overlay
- Rootfs assembly uses file-level copy (not overlayfs), which means:
  - Slower container creation for large images
  - Writable changes are stored in the merged rootfs directory (no separate upperdir)
  - No COW semantics — all writes go directly to the merged directory

### Image Concurrency
- The image pull flow is not thread-safe; concurrent pulls of the same image
  may cause race conditions with the index.json and layer directories.

### Pseudo-TTY Support
- The `-t` flag on `poqman run` is parsed but terminal raw mode handling
  is not yet implemented. Interactive sessions may not behave like a proper TTY.

### Architecture Support
- QEMU binary must be installed on the host for the target architecture.
- Cross-architecture emulation (e.g., running arm64 container on x86_64 host)
  requires the appropriate `qemu-system-*` binary to be available.
