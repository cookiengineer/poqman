# poqman — TODO

## Current State: All 7 Phases Complete

213 tests passing, 0 failures. `go vet` clean. `CGO_ENABLED=0` builds.

---

## Immediate Work Items (Next Sprint)

### 1. QEMU-based RUN Execution
**Problem:** `RUN` instructions in Dockerfiles are parsed but only recorded in image
history. They do not execute during the build. Packages installed via `RUN apt-get`
won't be present until the container first boots.

**Solution:** Implement the QEMU build VM:
- Create a build init script in rootfs that runs the command + `poweroff -f`
- Snapshot rootfs (file manifest) before RUN
- Boot QEMU with build rootfs via 9p, execute command in VM
- Wait for QEMU exit, read exit code
- Compute file diff → create layer from changed/added files
- Store layer in image

**Files:** `pkg/dockerfile/builder.go` (handleRun method)

### 2. Embed poqman-init + poqman-agent
**Problem:** `poqman-init` and `poqman-agent` are separate binaries. `poqman run`
currently uses a placeholder `getInitBinary()` that returns empty bytes.

**Solution:**
- Use `//go:embed` in main.go to embed the compiled init/agent binaries
- Write them to container rootfs at run time
- Cross-compile init/agent for all target architectures

**Files:** `cmd/poqman/main.go`, `cmd/poqman-init/main.go`, `cmd/poqman-agent/main.go`

### 3. Distribution Kernel Auto-Resolution
**Problem:** Debian/Alpine/Arch resolvers require exact package version strings.
Users must manually look up the full version.

**Solution:**
- Query packages.debian.org API for Debian package metadata
- Query pkgs.alpinelinux.org API for Alpine
- Query archive.archlinux.org for Arch
- Cache resolved URLs in kernel store

**Files:** `pkg/kernel/resolver_debian.go`, `resolver_alpine.go`, `resolver_archlinux.go`

---

## Remaining Tasks

| Task | Priority | Effort |
|------|----------|--------|
| QEMU-based RUN execution | HIGH | Large |
| Embed init/agent binaries | HIGH | Medium |
| Kernel auto-resolution | HIGH | Medium |
| System integration tests | MEDIUM | Large |
| iptables DNAT cleanup on container stop/rm | MEDIUM | Small |
| Agent socket readiness retry in exec | MEDIUM | Small |
| Thread-safe image pull (mutex on index.json) | MEDIUM | Small |
| Layer diffing for builds (file manifest) | MEDIUM | Medium |
| TTY raw mode for -it interactive attach | LOW | Small |
| poqman push to OCI registries | LOW | Large |
| Multi-stage builds (FROM ... AS) | LOW | Medium |
| Build layer caching | LOW | Large |
| Health checks | LOW | Medium |
| Docker Compose support | LOW | Large |
| IPv6 networking | LOW | Medium |
| podman save / load equivalent | LOW | Medium |

---

## Completed Phases

### Phase 1: Foundation ✅
Storage layout, Image/Container types, stores, CLI framework
Commands: `images`, `ps`
Tests: 26

### Phase 2: Registry & Pull ✅
OCI Distribution API, Docker Hub auth, manifest parsing, layer extraction
Commands: `pull [--platform]`
Tests: 20

### Phase 3: Kernel Store ✅
Kernel types, store, resolver registry, Debian/Alpine/Arch resolvers
Commands: `kernel pull|list|rm`
Tests: 17

### Phase 4: QEMU Runtime & Networking ✅
QEMU detection, arg builder, process lifecycle, QMP client
Bridge/TAP networking, iptables NAT, IPAM, poqman-init
Commands: `run`, `start`, `stop`, `logs`
Tests: 30

### Phase 5: Agent & Exec ✅
poqman-agent virtio-serial binary, AgentClient (Execute/Signal/Ping)
Commands: `exec`
Tests: 12

### Phase 6: Lifecycle Management ✅
Container/image removal with force flag, JSON inspection
Commands: `rm [-f]`, `rmi [-f]`, `inspect`
Tests: 13

### Phase 7: Build Engine ✅
Dockerfile scanner, parser (16 instruction types), builder
Commands: `build -t <tag> [-f <Dockerfile>] [--platform]`
Tests: 66 (44 parser + 22 builder)

---

## Implementation Notes

### Image Name Format
```
[registry/][namespace/]repo[:tag|@digest]
```
- `alpine` → docker.io/library/alpine:latest
- `nginx:1.25` → docker.io/library/nginx:1.25
- `myregistry.io/team/app:v1` → myregistry.io/team/app:v1
- `alpine@sha256:abc123` → docker.io/library/alpine@sha256:abc123
- `localhost:5000/myrepo:dev` → localhost:5000/myrepo:dev

### Container State Machine
```
created → running → stopped
                  → paused (future)
                  → failed
```

### QMP Protocol
- Connect: unix socket
- Greeting: read QMP capabilities response
- Capabilities: `{"execute": "qmp_capabilities"}`
- Shutdown: `{"execute": "system_powerdown"}`
- Force quit: `{"execute": "quit"}`
- Events: SHUTDOWN, RESET, POWERDOWN

### Build Requirements
- `CGO_ENABLED=0` for all binaries
- `go vet ./...` must pass
- No third-party dependencies (stdlib + golang.org/x only)
- All packages must have tests

### Test Commands
```bash
CGO_ENABLED=0 go test ./... -count=1 -cover
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 GOOS=linux go build ./cmd/poqman-init/
CGO_ENABLED=0 GOOS=linux go build ./cmd/poqman-agent/
```
