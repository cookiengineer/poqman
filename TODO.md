# poqman — TODO

## Current State

**All 10 phases complete.** 269 tests passing, 0 failures.
`go vet` clean. `CGO_ENABLED=0` builds for all binaries.
15 CLI commands. 11 packages + lifecycle test package.

```
CGO_ENABLED=0 go test ./... -count=1 -cover    # 269 tests, all passing
CGO_ENABLED=0 go vet ./...                      # clean
CGO_ENABLED=0 go build ./...                    # clean
make embed                                      # cross-compile init/agent for amd64+arm64
```

---

## Completed (Phases 1-10)

| Phase | Scope | Tests |
|-------|-------|-------|
| 1 | Foundation: stores, types, CLI framework, `images`, `ps` | 26 |
| 2 | Registry: OCI pull, Docker Hub auth, layer extraction, `pull` | 20 |
| 3 | Kernel: distro resolvers (Debian, Alpine, Arch), `kernel` | 17 |
| 4 | Runtime: QEMU, networking, poqman-init, `run`/`start`/`stop`/`logs` | 30 |
| 5 | Agent: virtio-serial, `exec` with 15s retry | 12 |
| 6 | Lifecycle: `rm [-f]`, `rmi [-f]`, `inspect` | 13 |
| 7 | Build: Dockerfile parser (17 instrs) + builder, `build` with QEMU RUN | 80 |
| 8 | Hardening: embed, DNAT cleanup, thread-safety, lifecycle e2e | 40 |
| 9 | Polish: kernel auto-resolution, diff tarballs, .dockerignore, TTY | 17 |
| 10 | Advanced: health checks, save/load, IPv6, DHCP, cgroups, system tests | 14 |
| **Total** | | **269** |

---

## Remaining Tasks

### Medium Priority

| Task | Effort | Notes |
|------|--------|-------|
| .dockerignore `**` globstar | Small | Deep recursive directory matching |
| Real QEMU integration tests | Large | Actual VM boots with kernel binaries |

### Low Priority / Nice to Have

| Task | Effort |
|------|--------|
| `poqman push` to OCI registries | Large |
| `poqman compose` | Large |
| Multi-stage builds (FROM ... AS + COPY --from) | Medium |
| Build layer caching | Large |
| Multi-arch init binary in single poqman binary | Medium |

---

## Implementation Notes

### CLI Commands (15)
```
build   exec    images   inspect   kernel   load    logs
ps      pull    rm       rmi       run      save    start   stop
```

### Image Name Format
```
[registry/][namespace/]repo[:tag|@digest]
```
- `alpine` → docker.io/library/alpine:latest
- `nginx:1.25` → docker.io/library/nginx:1.25
- `myregistry.io/team/app:v1` → myregistry.io/team/app:v1
- `localhost:5000/myrepo:dev` → localhost:5000/myrepo:dev

### Container State Machine
```
created → running → stopped
                  → paused (future)
                  → failed
```
Health: `starting` → `healthy` / `unhealthy`

### QMP Protocol
- Connect: unix socket
- Greeting: read QMP capabilities response
- Capabilities: `{"execute": "qmp_capabilities"}`
- Shutdown: `{"execute": "system_powerdown"}`
- Events: SHUTDOWN, RESET, POWERDOWN

### Build Requirements
- `CGO_ENABLED=0` for all binaries
- `go vet ./...` must pass
- No third-party dependencies (stdlib + golang.org/x only)
- All packages must have tests
- `make embed` before final build for pre-compiled init binaries

### Test Commands
```bash
CGO_ENABLED=0 go test ./... -count=1 -cover
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./...
make embed                     # build embedded init/agent before final build
make clean                     # remove build artifacts
```
