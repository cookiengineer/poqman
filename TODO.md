# poqman — TODO

## Current: Phase 6 — Lifecycle Management

### In Progress
- `pkg/cli/rm.go` — `poqman rm` + `poqman rmi`
- `pkg/cli/inspect.go` — `poqman inspect`

### Pending
- Tests for rm, rmi, inspect

---

## Completed Phases

### Phase 1: Foundation ✅
- XDG storage layout, Image/Container types, stores, CLI framework
- Commands: `images`, `ps`
- Tests: 26 tests (image parsing, store CRUD, container store, paths)

### Phase 2: Registry & Pull ✅
- OCI Distribution API client, Docker Hub auth, manifest parsing
- Platform matching, layer download + extraction + digest verification
- Commands: `pull [--platform]`
- Tests: 20 tests (auth, manifest, platform)

### Phase 3: Kernel Store ✅
- Kernel types, store, resolver registry, distribution resolvers (Debian, Alpine, Arch)
- Commands: `kernel pull|list|rm`
- Tests: 17 tests (kernel parsing, store CRUD, resolvers)

### Phase 4: QEMU Runtime & Networking ✅
- QEMU detection, arg builder, process lifecycle, QMP client
- Bridge/TAP networking, iptables NAT, IPAM
- poqman-init PID 1 binary
- Commands: `run`, `start`, `stop`, `logs`
- Tests: 30 tests (args, cmdline, MAC, arch mapping, agent)

### Phase 5: Agent & Exec ✅
- poqman-agent virtio-serial binary
- Host-side AgentClient (Execute, Signal, Ping)
- Commands: `exec`
- Tests: 13 tests (protocol, pipe, unix socket)

---

## Implementation Notes

### Image Name Format
```
[registry/][namespace/]repo[:tag|@digest]
```
Examples:
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

### QMP Protocol Notes
- Connect to unix socket
- First message: `{"execute": "qmp_capabilities"}`
- Graceful shutdown: `{"execute": "system_powerdown"}`
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
