# Springfield CSI Driver

**Minimal testing completed. NOT ready for use with real data.**

A prototype CSI driver backed by Blivet, rewritten in Go for improved performance and maintainability.

## Overview

This project is a proof of concept to build a CSI driver that uses a mapped DBus domain socket to create storage on the host and provide the storage for use to containers on demand.

Blivet has a prototype DBus API that provides high-level APIs to Linux storage technologies. LVM, BTRFS, MD, and Stratis are included in this prototype CSI driver code.

**Note:** There are known issues using BTRFS. See: https://github.com/springfield-csi/driver/issues/3

## Version

**v0.2** - Rewritten in Go from Python

## Features

- **CSI Specification Compliant**: Implements Identity, Controller, and Node services
- **Multiple Storage Backends**: Supports LVM, BTRFS, MD RAID, and Stratis
- **Blivet Integration**: Uses Blivet's DBus API for storage management
- **Topology Awareness**: Supports node topology constraints
- **Volume Lifecycle**: Create, delete, publish, unpublish, stage, unstage
- **Volume Stats**: Reports filesystem usage statistics
- **Online Expansion**: Supports online volume expansion capability (partial)

## Prerequisites

- Go 1.22 or later (for building)
- Kubernetes cluster (tested with kind)
- Blivet DBus server running on the host
- Block devices available for storage pool

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

**Note:** Every storage class must contain valid block devices. When the driver starts, it initializes all configured block devices. If any fail to initialize, the driver will exit.

### 3. Start Blivet DBus Server

Clone the `3.8-devel` branch from https://github.com/storaged-project/blivet.git:

```bash
git clone -b 3.8-devel https://github.com/storaged-project/blivet.git
cd blivet
export PYTHONPATH=$(pwd)
dbus/blivetd
```

**Note:** The `3.8-devel` branch has fixes necessary for the blivet DBus server to work properly.

**Known Issue:** If the CSI driver is restarted, the blivetd process must also be restarted.

### 4. Deploy the Driver

```bash
./deploy.sh
```

The driver will be deployed from `ghcr.io/springfield-csi/driver:devel` (or your configured registry).

### 5. Run Tests

```bash
cd tests
./lvm_test.sh
```

### 6. Access Test Pod

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
│   └── blivet/            # Blivet DBus client
├── deploy/
│   ├── docker/            # Old Python Dockerfile (deprecated)
│   └── helm/              # Helm charts for deployment
└── tests/                 # Test scripts and manifests
```

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
- Volume expansion partially implemented
- Clone volume not implemented
- No persistent volume state (in-memory only)
- BTRFS support has known issues
- Blivet DBus server must be restarted with the driver

## Development

### Project Structure

- `pkg/identity/`: Implements GetPluginInfo, GetPluginCapabilities, Probe
- `pkg/controller/`: Implements volume creation, deletion, and publishing
- `pkg/node/`: Implements volume staging, publishing, and statistics
- `pkg/blivet/`: DBus client for Blivet storage operations

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
