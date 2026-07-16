# poqman — ROADMAP

## Phases 1-10: Complete ✅
All 15 CLI commands. 259 tests (0 skips in unit tests).
Dockerfile parser with 17 instruction types. QEMU-based RUN execution.
Bridge + TAP networking with IPv6 + DHCP. Cgroup resource limits.
Image save/load. Health checks. System integration tests with real
QEMU VMs + actual Debian kernel package downloads.

## Phase 11: Kernel & Network Hardening

### Completed
- [x] **Debian resolver URL fix** — Changed from `pool/main/l/linux/` to
  `pool/main/l/linux-signed-amd64/` for signed kernel packages.
- [x] **Alpine resolver HTML parser fix** — Updated to match new package page
  structure. Added `stripHTMLTags()` for clean version extraction.
- [x] **Arch resolver fix** — Fixed package version parsing (arch suffix
  stripping, dot-separated versions after kernel version).
- [x] **Madison API fix** — Removed `&f=json` parameter; plain text format
  is parsed correctly by pipe-split logic.
- [x] **All kernel API tests use `t.Fatalf`** — No silent skips. 7 tests
  now fail loudly if network resources change.
- [x] **ImageIndex thread safety** — Added `sync.RWMutex` to prevent
  concurrent map access panics.

### In Progress / TODOs
- [ ] **Ubuntu kernel resolver** — New `resolver_ubuntu.go`. Package format
  is `.deb`, hosted at `archive.ubuntu.com`. Syntax: `KERNEL "ubuntu:6.8.0-50-generic"`.
- [ ] **Verify kernel API tests periodically** — The 7 network-dependent
  tests query live APIs. If Debian/Alpine/Arch remove packages or change
  their HTML structure, tests will fail (by design — no silent skip).

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
