# Recommended Architecture: Production-Ready CSI Driver

## Executive Summary

**Replace Blivet DBus with direct command execution using nsenter.**

This is the industry-standard approach used by:
- **OpenEBS LocalPV** - Direct LVM commands
- **TopoLVM** - lvmd daemon (optional)
- **Longhorn** - Direct device management
- **Portworx** - Custom storage engine

## Why This Approach?

### Current Problems with Blivet DBus

1. **Unreliable**: Blivet DBus server is buggy and experimental
2. **Complex**: Extra layer of abstraction adds failure points
3. **Stateful**: DBus maintains state that can desync from reality
4. **Python dependency**: Adds runtime overhead
5. **Coupling**: Driver restart requires Blivet restart

### Benefits of Direct Execution

1. **Reliable**: Standard Linux tools (lvm, mdadm, etc.)
2. **Simple**: Direct command execution, no middleware
3. **Stateless**: Commands query real state, no cached data
4. **Go-native**: Pure Go, no Python dependencies
5. **Debuggable**: Easy to see and reproduce commands

## Proposed Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                          │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  CSI Controller (StatefulSet)                        │    │
│  │  ┌──────────────────────────────────────────────┐   │    │
│  │  │  Identity/Controller Services                 │   │    │
│  │  └────────────────┬─────────────────────────────┘   │    │
│  │                   │                                   │    │
│  │  ┌────────────────▼─────────────────────────────┐   │    │
│  │  │  Storage Managers (pkg/storage)              │   │    │
│  │  │  ├─ LVM Manager                              │   │    │
│  │  │  ├─ MD RAID Manager                          │   │    │
│  │  │  ├─ BTRFS Manager                            │   │    │
│  │  │  └─ Stratis Manager                          │   │    │
│  │  └────────────────┬─────────────────────────────┘   │    │
│  │                   │                                   │    │
│  │  ┌────────────────▼─────────────────────────────┐   │    │
│  │  │  Command Executor (nsenter wrapper)          │   │    │
│  │  └──────────────────────────────────────────────┘   │    │
│  └─────────────────────┬───────────────────────────────┘    │
│                        │ nsenter                             │
│  ┌─────────────────────▼───────────────────────────────┐    │
│  │  Host Namespaces                                     │    │
│  │  ┌──────────────────────────────────────────────┐   │    │
│  │  │  lvm, mdadm, btrfs, stratis-cli              │   │    │
│  │  │  /dev/*, /sys/*, /proc/*                     │   │    │
│  │  └──────────────────────────────────────────────┘   │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  CSI Node (DaemonSet on each node)                  │    │
│  │  ┌──────────────────────────────────────────────┐   │    │
│  │  │  Node Service                                 │   │    │
│  │  │  ├─ Mount/Unmount                            │   │    │
│  │  │  ├─ Volume Stats                             │   │    │
│  │  │  └─ Health Monitoring                        │   │    │
│  │  └──────────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────────┘    │
└───────────────────────────────────────────────────────────────┘
```

## Code Structure

```
driver/
├── cmd/
│   └── csi-driver/
│       └── main.go                 # Server entrypoint
├── pkg/
│   ├── driver/
│   │   └── driver.go               # Constants, version
│   ├── identity/
│   │   └── identity.go             # CSI Identity service
│   ├── controller/
│   │   └── controller.go           # CSI Controller service (simplified)
│   ├── node/
│   │   └── node.go                 # CSI Node service
│   └── storage/                    # ⭐ NEW: Direct storage managers
│       ├── executor.go             # nsenter command executor
│       ├── lvm.go                  # LVM operations
│       ├── mdraid.go               # MD RAID operations
│       ├── btrfs.go                # BTRFS operations
│       └── stratis.go              # Stratis operations
├── deploy/
│   └── helm/
│       └── springfield-csi/
│           ├── templates/
│           │   ├── controller.yaml # Updated with host mounts
│           │   └── node.yaml       # Updated with host mounts
│           └── values.yaml
├── Dockerfile                      # Multi-stage, includes storage tools
├── Makefile
└── docs/
    ├── ARCHITECTURE_COMPARISON.md
    ├── REFACTORING_GUIDE.md
    └── RECOMMENDED_ARCHITECTURE.md
```

## Key Components

### 1. Storage Executor (pkg/storage/executor.go)

```go
// Executes commands in host namespaces
type Executor struct {
    nsenterPath  string
    hostProcPath string
}

func (e *Executor) Run(name string, args ...string) (string, error) {
    cmd := exec.Command(e.nsenterPath,
        "--mount=/host/proc/1/ns/mnt",
        "--ipc=/host/proc/1/ns/ipc",
        "--", name, args...)
    
    // Execute and return output
}
```

### 2. LVM Manager (pkg/storage/lvm.go)

```go
type LVMManager struct {
    executor *Executor
}

func (m *LVMManager) CreateLogicalVolume(vg, name, size string) (string, error) {
    _, err := m.executor.Run("lvcreate", "-n", name, "-L", size, vg)
    return fmt.Sprintf("/dev/%s/%s", vg, name), err
}

func (m *LVMManager) DeleteLogicalVolume(vg, name string) error {
    _, err := m.executor.Run("lvremove", "-f", fmt.Sprintf("%s/%s", vg, name))
    return err
}

func (m *LVMManager) CreateFilesystem(device, fsType string) error {
    cmd := fmt.Sprintf("mkfs.%s", fsType)
    _, err := m.executor.Run(cmd, "-f", device)
    return err
}
```

### 3. Controller (simplified)

```go
type Service struct {
    csi.UnimplementedControllerServer
    nodeID     string
    lvmMgr     *storage.LVMManager     // Direct managers
    mdMgr      *storage.MDManager
    btrfsMgr   *storage.BTRFSManager
    // ... no DBus client!
}

func (s *Service) CreateVolume(ctx, req) (*csi.CreateVolumeResponse, error) {
    // Parse request
    storageType := req.Parameters["storagetype"]  // "lvm", "mdraid", etc.
    
    // Direct execution
    var devicePath string
    switch storageType {
    case "lvm":
        devicePath, err = s.lvmMgr.CreateLogicalVolume(
            "csi-vg", req.Name, sizeStr)
        err = s.lvmMgr.CreateFilesystem(devicePath, fsType)
    case "mdraid":
        devicePath, err = s.mdMgr.CreateArray(req.Name, disks)
    // ... etc
    }
    
    // Return CSI response
}
```

## Deployment Configuration

### DaemonSet (Node)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: springfield-csi-node
spec:
  template:
    spec:
      hostNetwork: true
      hostPID: true
      containers:
      - name: csi-driver
        image: ghcr.io/springfield-csi/driver:v0.3
        args:
          - --endpoint=unix:///csi/csi.sock
          - --nodeonly
          - -v=4
        securityContext:
          privileged: true
        volumeMounts:
        # CSI socket
        - name: socket-dir
          mountPath: /csi
        # kubelet pods directory
        - name: pods-dir
          mountPath: /var/lib/kubelet/pods
          mountPropagation: Bidirectional
        # Host access for nsenter
        - name: host-dev
          mountPath: /host/dev
        - name: host-proc
          mountPath: /host/proc
          mountPropagation: HostToContainer
        - name: host-sys
          mountPath: /host/sys
      volumes:
      - name: socket-dir
        hostPath:
          path: /var/lib/kubelet/plugins/springfield.csi.redhat.com
          type: DirectoryOrCreate
      - name: pods-dir
        hostPath:
          path: /var/lib/kubelet/pods
          type: Directory
      - name: host-dev
        hostPath:
          path: /dev
          type: Directory
      - name: host-proc
        hostPath:
          path: /proc
          type: Directory
      - name: host-sys
        hostPath:
          path: /sys
          type: Directory
```

### StatefulSet (Controller)

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: springfield-csi-controller
spec:
  replicas: 1  # Single controller for now
  template:
    spec:
      containers:
      - name: csi-driver
        image: ghcr.io/springfield-csi/driver:v0.3
        args:
          - --endpoint=unix:///csi/csi.sock
          - -v=4
        securityContext:
          privileged: true
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
        # Same host mounts as node
        - name: host-dev
          mountPath: /host/dev
        - name: host-proc
          mountPath: /host/proc
        - name: host-sys
          mountPath: /host/sys
      # Standard CSI sidecars
      - name: csi-provisioner
        image: registry.k8s.io/sig-storage/csi-provisioner:v4.0.0
        args:
          - --csi-address=/csi/csi.sock
          - --feature-gates=Topology=true
        volumeMounts:
        - name: socket-dir
          mountPath: /csi
      - name: csi-attacher
        image: registry.k8s.io/sig-storage/csi-attacher:v4.5.0
        # ... similar config
      volumes:
      - name: socket-dir
        emptyDir: {}
      # ... host volumes same as node
```

## Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY pkg/ pkg/
RUN CGO_ENABLED=0 go build -o csi-driver ./cmd/csi-driver

# Runtime stage
FROM alpine:3.19

# Install storage management tools
RUN apk add --no-cache \
    ca-certificates \
    util-linux \      # nsenter, mount, umount, blkid
    lvm2 \            # lvm, pvcreate, vgcreate, lvcreate
    lvm2-extra \      # Additional LVM tools
    mdadm \           # mdadm for MD RAID
    btrfs-progs \     # mkfs.btrfs, btrfs
    xfsprogs \        # mkfs.xfs, xfs_growfs
    e2fsprogs \       # mkfs.ext4, resize2fs
    e2fsprogs-extra

# Optional: Install Stratis if available
# RUN apk add stratis-cli || true

COPY --from=builder /workspace/csi-driver /usr/local/bin/
RUN chmod +x /usr/local/bin/csi-driver

ENTRYPOINT ["/usr/local/bin/csi-driver"]
```

## Storage Class Examples

### LVM Storage Class

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: springfield-lvm
provisioner: springfield.csi.redhat.com
parameters:
  storagetype: lvm
  volumegroup: csi-vg-lvm
  fstype: xfs
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

### MD RAID Storage Class

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: springfield-mdraid
provisioner: springfield.csi.redhat.com
parameters:
  storagetype: mdraid
  raidlevel: raid1
  devices: /dev/sdb,/dev/sdc
  fstype: ext4
volumeBindingMode: WaitForFirstConsumer
```

## Testing Without Kubernetes

```bash
# Build the driver
make build

# Run directly on host (no container)
sudo bin/csi-driver \
  --endpoint=unix:///tmp/csi.sock \
  --nodeid=$(hostname) \
  -v=6

# In another terminal, test with csc tool
csc identity plugin-info --endpoint unix:///tmp/csi.sock

csc controller create-volume \
  --endpoint unix:///tmp/csi.sock \
  --cap SINGLE_NODE_WRITER,mount,xfs \
  --params storagetype=lvm,volumegroup=test-vg \
  test-volume-001
```

## Migration Path

### Phase 1: Create Storage Managers (1-2 days)
- [ ] Implement `pkg/storage/executor.go`
- [ ] Implement `pkg/storage/lvm.go`
- [ ] Add unit tests with mocked executor
- [ ] Test LVM operations manually

### Phase 2: Update Controller (1 day)
- [ ] Refactor controller to use LVM manager
- [ ] Remove Blivet client dependency
- [ ] Update error handling
- [ ] Test volume lifecycle

### Phase 3: Add Other Storage Types (2-3 days)
- [ ] Implement MD RAID manager
- [ ] Implement BTRFS manager
- [ ] Implement Stratis manager (optional)
- [ ] Update storage class templates

### Phase 4: Update Deployment (1 day)
- [ ] Update Dockerfile
- [ ] Update DaemonSet/StatefulSet YAMLs
- [ ] Update Helm chart
- [ ] Update documentation

### Phase 5: Testing (2-3 days)
- [ ] Test on kind cluster
- [ ] Test all storage types
- [ ] Test failure scenarios
- [ ] Performance testing

**Total estimated time: 1-2 weeks**

## Success Metrics

✅ **No Blivet dependency** - Can deploy without running Blivet DBus  
✅ **Faster startup** - < 500ms vs 2-3s  
✅ **Smaller image** - < 100MB vs 500MB  
✅ **Better logs** - See actual commands executed  
✅ **Easier debug** - Can reproduce commands manually  
✅ **Production-ready** - Same pattern as OpenEBS, TopoLVM  

## Conclusion

This architecture is:
- ✅ **Simpler** - Fewer moving parts
- ✅ **More reliable** - Standard Linux tools
- ✅ **Industry-standard** - Proven approach
- ✅ **Maintainable** - Pure Go, clear code
- ✅ **Debuggable** - Transparent operations

**Recommendation: Proceed with this refactoring.** The effort (1-2 weeks) is well worth it for a production-quality driver.
