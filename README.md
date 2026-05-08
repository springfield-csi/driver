# Springfield CSI Driver

**Minimal testing completed. NOT ready for use with real data.**

A prototype CSI driver for local storage, written in Go for performance and reliability.

## Overview

This project is a proof of concept CSI driver that manages local storage using direct command execution. It uses nsenter to run LVM, MD RAID, BTRFS, and Stratis commands on the host system.

The driver supports multiple storage backends without requiring external daemons or DBus services.

**Note:** There are known issues using BTRFS. See: https://github.com/springfield-csi/driver/issues/3

## Version

**v0.3** - Direct execution architecture (no Blivet dependency)  
**v0.2** - Initial Go rewrite from Python  
**v0.1** - Original Python implementation with Blivet

## Features

- **CSI Specification Compliant**: Implements Identity, Controller, and Node services
- **Multiple Storage Backends**: Supports LVM, BTRFS, MD RAID, and Stratis
- **Direct Execution**: Uses nsenter to run storage commands on the host
- **No External Daemons**: Self-contained, no Blivet or DBus required
- **Topology Awareness**: Supports node topology constraints
- **Volume Lifecycle**: Create, delete, publish, unpublish, stage, unstage
- **Volume Stats**: Reports filesystem usage statistics
- **Online Expansion**: Supports online volume expansion (LVM only)

## Prerequisites

- Go 1.22 or later (for building)
- Kubernetes cluster (tested with kind)
- Block devices available for storage
- Storage tools on host: lvm2, mdadm, btrfs-progs, xfsprogs

## Building

### Using Make

```bash
# Build binary
make build

# Build container image
make image

# Run tests
make test

# Format and lint code
make lint
```

### Using Go directly

```bash
# Download dependencies
go mod download

# Build the driver
go build -o bin/csi-driver ./cmd/csi-driver
```

## Installation

### 1. Setup Kind Cluster

```bash
./build_cluster.sh
```

### 2. Configure Storage Classes

Edit each of the storage class files and update the `disks` field:

```yaml
disks: /dev/XXX,/dev/XXX
```

**WARNING**: The CSI driver will wipe these devices clean! All data will be lost.

Storage class files to configure:
- `deploy/helm/springfield-csi/templates/storage-class-lvm.yaml`
- `deploy/helm/springfield-csi/templates/storage-class-md.yaml`
- `deploy/helm/springfield-csi/templates/storage-class-stratis.yaml`
- `deploy/helm/springfield-csi/templates/storage-class-btrfs.yaml`

**Note:** Every storage class must contain valid block devices. When the driver starts, it initializes volume groups/pools for the configured storage types.

### 3. Deploy the Driver

```bash
./deploy.sh
```

The driver will be deployed from `ghcr.io/springfield-csi/driver:devel` (or your configured registry).

**Note:** No Blivet DBus server is required! The driver executes commands directly on the host.

### 4. Run Tests

```bash
cd tests
./lvm_test.sh
```

### 5. Access Test Pod

```bash
kubectl exec --stdin --namespace default springfield-pvc-lvm-test -- /bin/bash
```

## Architecture

```
├── cmd/
│   └── csi-driver/        # Main entry point
├── pkg/
│   ├── driver/            # Driver constants and metadata
│   ├── identity/          # CSI Identity service
│   ├── controller/        # CSI Controller service
│   ├── node/              # CSI Node service
│   └── storage/           # Storage managers (LVM, MD, BTRFS, Stratis)
├── deploy/
│   ├── docker/            # Old Python Dockerfile (deprecated)
│   └── helm/              # Helm charts for deployment
├── docs/                  # Architecture documentation
└── tests/                 # Test scripts and manifests
```

## Improvements in v0.3 (Direct Execution)

- **No External Daemon**: Removed Blivet DBus dependency
- **Simpler Deployment**: No need to manage external services
- **More Reliable**: No DBus state synchronization issues
- **Better Performance**: Direct command execution, no marshalling
- **Easier Debugging**: See actual commands in logs
- **Stateless**: Commands query real state, no cached data
- **Industry Standard**: Same approach as OpenEBS and TopoLVM

## Improvements in v0.2 (Go Rewrite)

- **Type Safety**: Strong typing eliminates many runtime errors
- **Better Error Handling**: Explicit error returns and handling
- **Concurrency**: Goroutines and mutex-based synchronization
- **Memory Efficiency**: Reduced memory footprint
- **Standard Libraries**: Uses k8s.io/mount-utils for mount operations
- **Logging**: Structured logging with klog v2
- **Maintainability**: Cleaner code organization and better documentation

## Known Limitations

- Snapshots not implemented
- Volume expansion implemented for LVM only
- Clone volume not implemented
- No persistent volume state (in-memory only)
- BTRFS support has known issues
- Stratis support requires stratis-cli on host

## Development

### Project Structure

- `pkg/identity/`: Implements GetPluginInfo, GetPluginCapabilities, Probe
- `pkg/controller/`: Implements volume creation, deletion, and publishing
- `pkg/node/`: Implements volume staging, publishing, and statistics
- `pkg/storage/`: Direct execution managers for LVM, MD, BTRFS, Stratis

### Adding New Features

1. Update the appropriate service package
2. Add tests in the corresponding test file
3. Update documentation
4. Test with real storage devices (carefully!)

## Contributing

This is a prototype project. Contributions are welcome, but please note the experimental nature of the codebase.

## License

GNU General Public License v.2 or later

## Authors

Todd Gill <tgill@redhat.com>

## Links

- GitHub: https://github.com/springfield-csi/driver
- CSI Spec: https://github.com/container-storage-interface/spec
- Blivet: https://github.com/storaged-project/blivet
