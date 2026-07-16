# poqman — ROADMAP

## Phases 1-10: Complete ✅
All 15 CLI commands. 251 unit tests (0 skips) + integration tests.
Dockerfile parser with 17 instruction types. QEMU-based RUN execution.
Bridge + TAP networking with IPv6 + DHCP. Cgroup resource limits.
Image save/load. Health checks. System integration tests with real
QEMU VMs + actual Debian kernel package downloads.

## Phase 11: Kernel & Network Hardening

### Completed
All items finished. 251 unit tests, 0 skips. Four Linux distros supported.

- [x] **Debian resolver URL fix** — `pool/main/l/linux-signed-amd64/` for signed packages.
- [x] **Alpine resolver HTML parser fix** — Updated for new package page + `stripHTMLTags()`.
- [x] **Arch resolver fix** — Package version parsing with arch suffix stripping.
- [x] **Madison API fix** — Removed `&f=json`; plain text pipe format.
- [x] **All kernel API tests use `t.Fatalf`** — 10 tests, 0 silent skips.
- [x] **ImageIndex thread safety** — `sync.RWMutex` on index map.
- [x] **Ubuntu kernel resolver** — `resolver_ubuntu.go`. Auto-resolution via
  `archive.ubuntu.com` pool scraper. `KERNEL "ubuntu:7.1.0-5-generic"`.
  Zstd extraction for modern Ubuntu .deb packages.
- [x] **Ubuntu LTS + kernel boot test** — Full lifecycle: FROM ubuntu:latest →
  KERNEL ubuntu → build → extract 17MB kernel → boot VM → verify
  `Linux version 7.1.0-5-generic #5-Ubuntu`.

## Phase 12: Remaining Work

### Medium Priority
- [ ] Real VM boot integration tests with all three distro kernels
- [ ] `.dockerignore` `**` globstar support

### Low Priority
- [ ] `poqman push` — Push images to OCI registries
- [ ] `poqman compose` — docker-compose.yml support
- [ ] Multi-stage builds (FROM ... AS + COPY --from)
- [ ] Build layer caching
- [ ] Fedora/RHEL kernel resolver

---

## Known Limitations

### Distribution Kernel Resolvers
- Auto-resolution queries live APIs. If an API changes (page structure, URL),
  the resolver falls back to explicit version format.
- Debian kernels may lack `CONFIG_9P_FS` — the build VM handles this
  gracefully via `panic=1` + `-no-reboot` timeout.

### RUN Instruction During Build
- QEMU build VM boots with 9p rootfs. If kernel lacks 9p support (common
  in distro kernels), RUN falls back to recording-only mode.
- Each RUN boots a full VM. Chain commands with `&&` for efficiency.

### Networking
- Bridge requires `iproute2` + `iptables`. IPv6 via ULA subnet.
- DHCP requires dnsmasq installed on host.

### Cgroups
- `ApplyCGroupLimits` requires root or cgroup v2 delegation.
