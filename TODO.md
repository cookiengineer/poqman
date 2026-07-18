# poqman — TODO

## Current State

**All features implemented.** 251 unit tests + 13 integration tests.
**0 skips** in unit tests — all kernel API tests use `t.Fatalf`.
`go vet` clean. `CGO_ENABLED=0` builds for all binaries.

**Active**: Initrd support for distro kernels (Phase 12).

```
CGO_ENABLED=0 go test $(go list ./... | grep -v lifecycle) -count=1    # 251 tests, 0 skips
CGO_ENABLED=0 go test ./pkg/lifecycle/ -count=1                         # 13 integration tests
CGO_ENABLED=0 go vet ./...                                               # clean
CGO_ENABLED=0 go build ./...                                             # clean
```

---

## Completed (Phases 1-10)

| Phase | Scope | Tests |
|-------|-------|-------|
| 1 | Foundation: stores, types, CLI framework | 26 |
| 2 | Registry: OCI pull, Docker Hub auth | 20 |
| 3 | Kernel: distro resolvers (Debian, Alpine, Arch, Ubuntu) | 27 |
| 4 | Runtime: QEMU, networking, poqman-init | 30 |
| 5 | Agent: virtio-serial exec | 12 |
| 6 | Lifecycle: rm/rmi/inspect | 13 |
| 7 | Build: Dockerfile parser + builder | 92 |
| 8 | Hardening: embed, DNAT, thread-safety | 40 |
| 9 | Polish: .dockerignore, diff tarballs, TTY, kernel auto-resolution | 17 |
| 10 | Advanced: health checks, save/load, IPv6, DHCP, cgroups, system tests | 14 |
| **Total** | | **287** (247 unit + 12 QEMU/Dockerfile integration) |

---

## Remaining Tasks

### Medium Priority
- [ ] **Initrd support for distro kernels** (see Phase 12 in ROADMAP.md)
  - [ ] Extract 9p kernel modules during kernel pull
  - [ ] Build minimal cpio.gz initrd with busybox + 9p .ko modules
  - [ ] Pass `-initrd` to QEMU in build VM (builder.go)
  - [ ] Pass `-initrd` to QEMU in run VM (run.go)
  - [ ] Adjust kernel cmdline when initrd is used (omit root=/rootfstype=)
- [ ] `.dockerignore` `**` globstar deep recursive matching
- [ ] Fedora/RHEL kernel resolver

### Low Priority
| Task | Effort |
|------|--------|
| `poqman push` | Large |
| `poqman compose` | Large |
| Multi-stage builds | Medium |
| Build layer caching | Large |

---

## Implementation Notes

### Image Name Format
```
[registry/][namespace/]repo[:tag|@digest]
```

### Container State Machine
```
created → running → stopped → failed
```

### QMP Protocol
- `{"execute": "qmp_capabilities"}`
- `{"execute": "system_powerdown"}`
- Events: SHUTDOWN, RESET, POWERDOWN

### Build Requirements
- `CGO_ENABLED=0`, `go vet`, no third-party deps

### Test Commands
```bash
CGO_ENABLED=0 go test $(go list ./... | grep -v lifecycle) -count=1    # units
CGO_ENABLED=0 go test ./pkg/lifecycle/ -count=1 -timeout 600s          # integration
```
