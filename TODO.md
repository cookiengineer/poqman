# poqman — TODO

## Current State

**All features implemented.** 247 unit tests + 12 QEMU/Dockerfile integration tests = **259 total**.
**0 skips** in unit tests. All kernel API tests (`t.Fatalf` instead of `t.Skipf`).
`go vet` clean. `CGO_ENABLED=0` builds for all binaries.

```
CGO_ENABLED=0 go test $(go list ./... | grep -v lifecycle) -count=1    # 247 tests, 0 skips
CGO_ENABLED=0 go test ./pkg/lifecycle/ -count=1 -run "Qemu|Dockerfile"  # 12 integration tests
CGO_ENABLED=0 go vet ./...                                               # clean
CGO_ENABLED=0 go build ./...                                             # clean
```

---

## Completed (Phases 1-10)

| Phase | Scope | Tests |
|-------|-------|-------|
| 1 | Foundation: stores, types, CLI framework | 26 |
| 2 | Registry: OCI pull, Docker Hub auth | 20 |
| 3 | Kernel: distro resolvers (Debian, Alpine, Arch) | 23 |
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

### High Priority — Network Dependency Verification
- [ ] **Verify all kernel API tests remain non-skipping** — The resolvers query
  external APIs (debian.org, alpinelinux.org, archlinux.org). If these APIs
  change or packages are removed, tests will fail. Need monitoring.
  - `TestResolveDebianPackage_Real` — madison API, uses `6.1.0-50-amd64:6.1.176-1`
  - `TestResolveAlpinePackage_Real` — pkgs.alpinelinux.org, uses `3.21/lts`
  - `TestResolveArchPackage_Real` — archive.archlinux.org, uses `6.9.9`
  - All use `t.Fatalf` (fail hard, no silent skip)

- [ ] **Ubuntu kernel support** — Add `resolver_ubuntu.go` with Ubuntu kernel
  package resolution. Ubuntu uses `.deb` packages hosted at `archive.ubuntu.com`.
  Kernel packages follow `linux-image-{version}-generic` naming.
  - KERNEL syntax: `ubuntu:6.8.0-50-generic` or `ubuntu:6.8.0-50:generic`
  - Package URL: `http://archive.ubuntu.com/ubuntu/pool/main/l/linux/`
  - Or use Launchpad API for package metadata lookup

### Medium Priority
- [ ] System integration: real VM boot with all three distro kernels
- [ ] `.dockerignore` `**` globstar deep recursive matching

### Low Priority
| Task | Effort |
|------|--------|
| `poqman push` | Large |
| `poqman compose` | Large |
| Multi-stage builds (FROM ... AS + COPY --from) | Medium |
| Build layer caching | Large |
| Fedora/RHEL kernel resolver | Medium |

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
