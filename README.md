:construction: LLM generated Prototype :construction:

# poqman

poqman - podman, but for qemu.

Dockerfile-compatible containers powered by QEMU microVMs. Name inspired by the extinct [Poq Bird](https://en.wikipedia.org/wiki/Atitl%C3%A1n_grebe).

<img src="doc/image.jpg" width="600" alt="poqman logo">

poqman builds and runs fully-emulated containers using QEMU as the isolation layer. Each container boots a custom Linux kernel in a lightweight VM with a root filesystem assembled from standard OCI/Docker image layers — just like podman, but with kernel-level isolation via hardware virtualization.

## Quick Start

```bash
# Pull a Debian base image
poqman pull debian:bookworm-slim

# Build with a pinned Debian kernel
cat > Dockerfile << 'EOF'
FROM debian:bookworm-slim
KERNEL "debian:6.1.0-25-amd64:6.1.106-3"
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends nginx \
    && rm -rf /var/lib/apt/lists/*
COPY index.html /var/www/html/
EXPOSE 80/tcp
CMD ["nginx", "-g", "daemon off;"]
EOF

echo '<h1>Hello from poqman</h1>' > index.html

poqman build -t myapp:latest .

# Run the container (detached, with port forwarding)
poqman run -d -p 8080:80 --name webserver myapp:latest

# See what's running
poqman ps

# Execute commands inside the running container
poqman exec webserver cat /etc/debian_version

# Inspect
poqman inspect myapp:latest

# Stop and clean up
poqman stop webserver
poqman rm webserver
```

## Supported Distributions for Custom Kernels

The `KERNEL` instruction downloads and bundles distribution-specific kernel packages:

| Distro | Syntax | Example |
|--------|--------|---------|
| Debian | `debian:<version>:<pkg-version>` | `debian:6.1.0-25-amd64:6.1.106-3` |
| Alpine | `alpine:<release>:<flavor>:<version>` | `alpine:3.21:lts:6.6.52-0-lts` |
| Arch Linux | `archlinux:<version>:<pkg-version>` | `archlinux:6.10.10:arch1-1` |
| OCI image | `docker.io/repo/kernel:tag` | `docker.io/poqman/kernel-debian:6.1` |
| Direct URL | `https://...` | `https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.1.tar.xz` |

## Commands

```
build        Build an image from a Dockerfile
exec         Execute a command in a running container
images       List images in local storage
inspect      Display detailed information on containers or images
kernel       Manage kernel images
logs         Fetch the logs of a container
ps           List containers
pull         Pull an image from a registry
rm           Remove one or more containers
rmi          Remove one or more images
run          Run a command in a new container
start        Start one or more stopped containers
stop         Stop one or more running containers
```

## How It Works

Each container is a QEMU microVM booting its own kernel and mounting the container root filesystem via virtio-9p. A tiny Go init binary (`poqman-init`) runs as PID 1 inside the VM, handling process lifecycle, networking, and volume mounts.

```
┌─────────────────────────────────────────────────┐
│  poqman run debian:bookworm-slim                 │
│                                                   │
│  ┌─────────────────────────────────────────────┐ │
│  │ QEMU microVM                                 │ │
│  │  Kernel:  bzImage (Debian 6.1.0-25)         │ │
│  │  PID 1:   /sbin/init (poqman-init)          │ │
│  │  Rootfs:  virtio-9p (Debian layers)          │ │
│  │  Network: virtio-net → tap → bridge → NAT    │ │
│  │  Exec:    virtio-serial → poqman-agent       │ │
│  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

## Requirements

- Linux host with QEMU (`qemu-system-x86_64` or other arch)
- `iproute2` and `iptables` for container networking
- Go 1.21+ (build only)
- No root required (except for bridge/NAT setup on first run)

## Build

```bash
CGO_ENABLED=0 go build -o bin/poqman ./cmd/poqman/
CGO_ENABLED=0 GOOS=linux go build -o bin/poqman-init ./cmd/poqman-init/
CGO_ENABLED=0 GOOS=linux go build -o bin/poqman-agent ./cmd/poqman-agent/
```

Zero third-party dependencies. Standard library and `golang.org/x` only.

## Test

```bash
CGO_ENABLED=0 go test ./... -count=1 -cover
```

213 tests, 0 failures across all packages.

## Storage

All data lives under `~/.local/share/poqman/` (or `$XDG_DATA_HOME/poqman/`):

```
~/.local/share/poqman/
├── images/         # Pulled OCI images + extracted layers
├── kernels/        # Downloaded distribution kernel packages
├── containers/     # Container configs, rootfs, logs, sockets
├── networks/       # Bridge + IP allocation state
└── tmp/            # Build staging
```
