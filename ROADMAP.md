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

## Phase 12: Initrd Support for Distro Kernels

### Motivation

Distro kernels (Debian, Ubuntu) compile 9p filesystem support as modules
(`CONFIG_9P_FS=m`), not built-in (`=y`). The kernel cannot mount `root=rootfs
rootfstype=9p` without an initrd that loads the 9p modules first. This blocks
`poqman build` RUN commands and `poqman run` on Debian/Ubuntu kernels.

### Implementation Plan

- [ ] **Extract 9p modules during kernel pull** (`pkg/kernel/puller.go`):
  In `downloadAndExtract()`, find `lib/modules/<version>/kernel/fs/9p/`
  and `net/9p/` in the extracted package tree. Copy .ko files to
  `kernels/<id>/modules/<version>/kernel/`. Store module version in
  `Kernel` struct for initrd assembly.

- [ ] **Add module storage paths** (`pkg/storage/paths.go`):
  `KernelModulesDir(id)` → `kernels/<id>/modules/`
  `ContainerInitrdPath(id)` → `containers/<id>/initrd.gz`

- [ ] **Initrd builder** (`pkg/kernel/initrd.go` — new file):
  `BuildInitrd(kernelID string, outputPath string, initBinary string)`
  - Detect if kernel has 9p modules at `KernelModulesDir`
  - Find busybox on host (try `/bin/busybox`, fallback check)
  - Create temp tree: `bin/{busybox,sh,mount,umount,insmod,switch_root}`
  - Copy 9p .ko files to `lib/modules/` in tree
  - Write init script that loads modules → mounts 9p → switch_root
  - Pack via `find . | cpio -o -H newc | gzip > initrd.gz`
  - Return nil if no modules exist (caller boot without initrd)

- [ ] **Use initrd in build VM** (`pkg/dockerfile/builder.go`):
  In `handleRun()`, call `kernel.BuildInitrd()` before booting VM.
  Pass `-initrd <path>` to QEMU. Strip `root=rootfs rootfstype=9p rootflags=...`
  from kernel cmdline (keep `init=`, `console=`, `panic=`).

- [ ] **Use initrd in run VM** (`pkg/cli/run.go`):
  In `RegisterRun()`, after assembling rootfs and injecting init,
  call `kernel.BuildInitrd()` for the container's kernel. Pass
  `-initrd` via `QEMUConfig.Initrd`. Adjust `BuildKernelAppend()`
  to omit 9p root params when initrd is used.

- [ ] **Adjust kernel cmdline builder** (`pkg/runtime/args.go`):
  Add `UseInitrd bool` to `QEMUConfig`. When true, `BuildKernelAppend()`
  omits `root=rootfs rootfstype=9p rootflags=...` since initrd handles
  the mount.

### Completed
- [x] Image name normalization in build (FullName)
- [x] Kernel panic detection in build VM
- [x] Fix: missing exit code file now fails the build

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
- Debian kernels may lack `CONFIG_9P_FS` — handled by automatic initrd generation
  that loads 9p modules before mounting the root filesystem.

### RUN Instruction During Build
- QEMU build VM boots with 9p rootfs. If kernel lacks 9p support, an initrd
  is automatically generated and passed to QEMU to load modules first.
- Each RUN boots a full VM. Chain commands with `&&` for efficiency.

### Networking
- Bridge requires `iproute2` + `iptables`. IPv6 via ULA subnet.
- DHCP requires dnsmasq installed on host.

### Cgroups
- `ApplyCGroupLimits` requires root or cgroup v2 delegation.
