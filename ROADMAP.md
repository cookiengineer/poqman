# poqman — ROADMAP

## Phases 1-7: Core ✅
All base CLI commands, OCI registry pull, Dockerfile parser + builder with QEMU RUN,
bridge networking, QMP lifecycle, virtio-serial exec agent.
Foundational test suite (238 tests).

## Phase 8: Hardening ✅
- QEMU-based RUN execution with snapshot/diff/layer
- poqman-init embedded via go:embed + shell fallback
- Agent socket retry (15s timeout)
- iptables DNAT cleanup on stop/force kill
- Thread-safe image index (RWMutex)
- End-to-end lifecycle tests (pkg/lifecycle/)

## Phase 9: Polish ✅
- Distribution kernel auto-resolution (Debian madison, Alpine, Arch APIs)
- Layer diff tarballs (createDiffLayer, extractLayerFile)
- .dockerignore support (wildcard, directory, negation, basename)
- TTY raw mode for `-it` via golang.org/x/term

## Phase 10: Advanced Features ✅
- **Health checks** — HEALTHCHECK Dockerfile instruction, HealthConfig/HealthState types,
  starting/healthy/unhealthy status tracking
- **Image save/load** — tar.gz export/import with manifest.json + layers + kernel.
  `poqman save [-o <file>]` and `poqman load -i <file>`
- **IPv6 networking** — Dual-stack bridge (`fd00:dead:beef::/64`), AllocateIPv6,
  SetupIPv6 with forwarding
- **DHCP support** — dnsmasq integration for dynamic IP assignment
- **Resource limits** — Cgroup-based ApplyCGroupLimits (memory.max, cpu.weight, pids.max),
  QEMU `-m` / `-smp` integration
- **System integration tests** — QEMU detection, version, full args validation,
  console device mapping, save/load round-trip (14 tests in pkg/lifecycle/)

## Phase 11: Remaining Work

### Medium Priority
- [ ] **`.dockerignore` globstar (`**`)** — Deep recursive directory matching
- [ ] **ACTUAL integration tests** — Real QEMU VM boot with actual kernel binaries

### Low Priority / Nice to Have
- [ ] `poqman push` — Push images to OCI registries
- [ ] `poqman compose` — docker-compose.yml support
- [ ] Multi-stage builds (FROM ... AS + COPY --from=stage)
- [ ] Build layer caching
- [ ] Multi-architecture init binary embedding in single poqman binary

---

## Known Limitations & Workarounds

### Distribution Kernel Resolvers
- Auto-resolution available for Debian, Alpine, Arch. Falls back to explicit
  version format if API calls fail (network issue, API change).

### RUN Instruction During Build
- `RUN` executes via QEMU only if KERNEL specified before it and QEMU available.
  Otherwise falls back to recording-only (runs at container startup).
- Each RUN boots a full VM. Chain commands with `&&` for efficiency.

### Networking
- Bridge requires `iproute2` and `iptables` on host. IP forwarding must be enabled.
- IPv6 supported via ULA subnet; requires host IPv6 forwarding enabled.
- DHCP requires dnsmasq installed on host.

### Rootfs Overlay
- File-level copy (not overlayfs). Writable changes go directly into merged directory.

### Cgroups
- `ApplyCGroupLimits` requires root or appropriate cgroup delegation (cgroups v2).

### Architecture Support
- QEMU binary must be installed for the target architecture.
- Cross-architecture emulation requires appropriate `qemu-system-*` binary.

### Health Checks
- HEALTHCHECK instruction parsed and stored in image config.
- Runtime health check execution via agent not yet wired into container lifecycle.
- Health status can be tracked via the `HealthState` type.
