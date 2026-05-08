# Refactoring Guide: Remove Blivet DBus Dependency

This guide shows how to refactor the CSI driver to use direct command execution instead of Blivet DBus.

## Step-by-Step Refactoring

### Step 1: Update Dependencies

**go.mod** - Remove DBus, add exec utilities
```diff
 require (
     github.com/container-storage-interface/spec v1.9.0
-    github.com/godbus/dbus/v5 v5.1.0
     github.com/kubernetes-csi/csi-lib-utils v0.18.0
     google.golang.org/grpc v1.62.1
     google.golang.org/protobuf v1.33.0
     k8s.io/klog/v2 v2.120.1
     k8s.io/mount-utils v0.29.3
+    k8s.io/utils v0.0.0-20230726121419-3b25d923346b
 )
```

### Step 2: Create Storage Managers

Create `pkg/storage/` with managers for each storage type:

```
pkg/storage/
├── executor.go      # Common command execution with nsenter
├── lvm.go          # LVM operations
├── mdraid.go       # MD RAID operations
├── btrfs.go        # BTRFS operations
└── stratis.go      # Stratis operations
```

**pkg/storage/executor.go**
```go
package storage

import (
    "bytes"
    "fmt"
    "os/exec"
    "k8s.io/klog/v2"
)

// Executor runs commands on the host using nsenter
type Executor struct {
    nsenterPath string
    hostProcPath string
}

func NewExecutor() *Executor {
    return &Executor{
        nsenterPath: "/usr/bin/nsenter",
        hostProcPath: "/host/proc",
    }
}

func (e *Executor) Run(name string, args ...string) (string, error) {
    // Prepend nsenter to run in host namespaces
    fullArgs := []string{
        fmt.Sprintf("--mount=%s/1/ns/mnt", e.hostProcPath),
        fmt.Sprintf("--ipc=%s/1/ns/ipc", e.hostProcPath),
        "--",
        name,
    }
    fullArgs = append(fullArgs, args...)
    
    cmd := exec.Command(e.nsenterPath, fullArgs...)
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    klog.V(4).Infof("Executing: %s %v", name, args)
    
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("%s failed: %s (stderr: %s)", 
            name, err, stderr.String())
    }
    
    return stdout.String(), nil
}
```

### Step 3: Update Controller Service

**Before (using Blivet):**
```go
type Service struct {
    csi.UnimplementedControllerServer
    nodeID      string
    blivetClient *blivet.Client  // ❌ Remove this
    volumes     map[string]*VolumeInfo
    volumesMu   sync.RWMutex
}

func New(nodeID string, blivetClient *blivet.Client) *Service {
    return &Service{
        nodeID:      nodeID,
        blivetClient: blivetClient,  // ❌ Remove this
        volumes:     make(map[string]*VolumeInfo),
    }
}

func (s *Service) Setup() error {
    return s.blivetClient.Reset()  // ❌ Remove this
}
```

**After (using direct execution):**
```go
type Service struct {
    csi.UnimplementedControllerServer
    nodeID      string
    lvmMgr      *storage.LVMManager      // ✅ Add this
    mdMgr       *storage.MDManager       // ✅ Add this
    btrfsMgr    *storage.BTRFSManager    // ✅ Add this
    stratisMgr  *storage.StratisManager  // ✅ Add this
    volumes     map[string]*VolumeInfo
    volumesMu   sync.RWMutex
}

func New(nodeID string) *Service {
    executor := storage.NewExecutor()
    
    return &Service{
        nodeID:     nodeID,
        lvmMgr:     storage.NewLVMManager(executor),     // ✅ Add this
        mdMgr:      storage.NewMDManager(executor),      // ✅ Add this
        btrfsMgr:   storage.NewBTRFSManager(executor),   // ✅ Add this
        stratisMgr: storage.NewStratisManager(executor), // ✅ Add this
        volumes:    make(map[string]*VolumeInfo),
    }
}

func (s *Service) Setup() error {
    // No setup needed - stateless!
    return nil
}
```

### Step 4: Update CreateVolume

**Before (Blivet):**
```go
func (s *Service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
    // ... validation ...
    
    shortName := shortenName(req.GetName())
    
    // ❌ Complex DBus call
    objPath, blockPath, err := s.blivetClient.CreateVolume(
        shortName, disks, blivetType, defaultVolumeSize)
    if err != nil {
        return nil, status.Errorf(codes.Internal, 
            "Failed to create volume: %v", err)
    }
    
    volInfo := &VolumeInfo{
        Name:          req.GetName(),
        CSIVolumeID:   req.GetName(),
        ObjectPath:    objPath,      // ❌ DBus specific
        BlockPath:     blockPath,
        CapacityBytes: size,
        Disks:         disks,
        StorageType:   blivetType,
        FSType:        fsType,
    }
    
    // ...
}
```

**After (Direct execution):**
```go
func (s *Service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
    // ... validation ...
    
    shortName := shortenName(req.GetName())
    
    var devicePath string
    var err error
    
    // ✅ Simple, direct calls
    switch storageType {
    case "lvm":
        // Ensure VG exists
        vgName := "csi-vg-lvm"
        if err := s.lvmMgr.EnsureVolumeGroup(vgName, disks); err != nil {
            return nil, status.Errorf(codes.Internal, 
                "Failed to ensure VG: %v", err)
        }
        
        // Create LV
        devicePath, err = s.lvmMgr.CreateLogicalVolume(
            vgName, shortName, sizeStr)
        if err != nil {
            return nil, status.Errorf(codes.Internal, 
                "Failed to create LV: %v", err)
        }
        
        // Create filesystem
        if err := s.lvmMgr.CreateFilesystem(devicePath, fsType); err != nil {
            s.lvmMgr.DeleteLogicalVolume(vgName, shortName) // Cleanup
            return nil, status.Errorf(codes.Internal, 
                "Failed to create filesystem: %v", err)
        }
        
    case "mdraid":
        devicePath, err = s.mdMgr.CreateArray(shortName, disks, "raid1")
        // ... similar pattern ...
        
    case "btrfs":
        devicePath, err = s.btrfsMgr.CreateVolume(shortName, disks)
        // ... similar pattern ...
        
    case "stratis":
        devicePath, err = s.stratisMgr.CreateFilesystem(shortName, disks)
        // ... similar pattern ...
    }
    
    volInfo := &VolumeInfo{
        Name:          req.GetName(),
        CSIVolumeID:   req.GetName(),
        BlockPath:     devicePath,    // ✅ Simple device path
        CapacityBytes: size,
        Disks:         disks,
        StorageType:   storageType,
        FSType:        fsType,
    }
    
    // ...
}
```

### Step 5: Update DeleteVolume

**Before (Blivet):**
```go
func (s *Service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
    // ...
    
    // ❌ Complex DBus operations
    if err := s.blivetClient.DeleteVolume(vol.ObjectPath, vol.Disks); err != nil {
        return nil, status.Errorf(codes.Internal, 
            "Failed to delete volume: %v", err)
    }
    
    // ...
}
```

**After (Direct execution):**
```go
func (s *Service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
    // ...
    
    // ✅ Simple, direct call
    switch vol.StorageType {
    case "lvm":
        vgName := "csi-vg-lvm"
        shortName := shortenName(vol.Name)
        if err := s.lvmMgr.DeleteLogicalVolume(vgName, shortName); err != nil {
            return nil, status.Errorf(codes.Internal, 
                "Failed to delete LV: %v", err)
        }
        
    case "mdraid":
        if err := s.mdMgr.DeleteArray(vol.Name); err != nil {
            return nil, status.Errorf(codes.Internal, 
                "Failed to delete MD array: %v", err)
        }
        
    // ... similar for btrfs, stratis ...
    }
    
    // ...
}
```

### Step 6: Update Main

**Before (Blivet):**
```go
func run() error {
    // ...
    
    if !*nodeOnly {
        // ❌ Create DBus client
        blivetClient, err := blivet.NewClient()
        if err != nil {
            return fmt.Errorf("failed to create Blivet client: %w", err)
        }
        defer blivetClient.Close()
        
        controllerSvc := controller.New(*nodeID, blivetClient)
        // ...
    }
    
    // ...
}
```

**After (Direct execution):**
```go
func run() error {
    // ...
    
    if !*nodeOnly {
        // ✅ No external client needed
        controllerSvc := controller.New(*nodeID)
        csi.RegisterControllerServer(grpcServer, controllerSvc)
        klog.Info("Controller service registered")
        
        // ✅ No setup errors to worry about
        if err := controllerSvc.Setup(); err != nil {
            klog.Warningf("Controller setup warning: %v", err)
        }
    }
    
    // ...
}
```

### Step 7: Update Dockerfile

**Before (with DBus):**
```dockerfile
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    dbus \           # ❌ Remove
    dbus-libs \      # ❌ Remove
    # ...
```

**After (direct execution):**
```dockerfile
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    util-linux \     # ✅ nsenter, mount
    lvm2 \           # ✅ LVM tools
    mdadm \          # ✅ MD tools
    btrfs-progs \    # ✅ BTRFS tools
    xfsprogs \       # ✅ XFS tools
    e2fsprogs        # ✅ ext4 tools
```

### Step 8: Update DaemonSet

**Before:**
```yaml
# No special mounts needed for DBus
```

**After:**
```yaml
apiVersion: apps/v1
kind: DaemonSet
spec:
  template:
    spec:
      containers:
      - name: springfield-csi-node
        securityContext:
          privileged: true  # Required for nsenter
        volumeMounts:
        # ✅ Mount host directories for nsenter
        - name: host-dev
          mountPath: /host/dev
        - name: host-proc
          mountPath: /host/proc
        - name: host-sys
          mountPath: /host/sys
      volumes:
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

## Testing the Refactored Code

```bash
# 1. Build new image
make image IMAGE_TAG=no-dbus

# 2. Update deployment to use new image
# Edit deploy/helm/springfield-csi/values.yaml

# 3. No need to start Blivet DBus server! 🎉

# 4. Deploy
./deploy.sh

# 5. Test volume creation
kubectl apply -f tests/lvm_test.yaml

# 6. Check logs
kubectl logs -l app=springfield-csi-controller -f
```

## Benefits After Refactoring

✅ **Removed dependencies:**
- No Blivet DBus server
- No Python runtime
- No dbus libraries

✅ **Simplified deployment:**
- One less service to manage
- No restart synchronization
- No DBus socket mounting

✅ **Improved reliability:**
- No DBus state to get out of sync
- Direct command execution
- Better error messages

✅ **Easier debugging:**
- See exact commands in logs
- Can run commands manually to test
- Standard tools (lvm, mdadm)

✅ **Better performance:**
- No DBus marshalling overhead
- Direct system calls
- Faster startup (no DBus connection)

## Migration Checklist

- [ ] Create `pkg/storage/` package
- [ ] Implement LVM manager with nsenter
- [ ] Update controller to use LVM manager
- [ ] Test LVM operations
- [ ] Implement MD manager
- [ ] Implement BTRFS manager
- [ ] Implement Stratis manager
- [ ] Remove `pkg/blivet/` package
- [ ] Update Dockerfile
- [ ] Update DaemonSet YAML
- [ ] Update documentation
- [ ] Remove Blivet setup instructions

## Estimated Effort

- **LVM manager**: 2-3 hours
- **Controller refactor**: 2-3 hours
- **Testing**: 2-4 hours
- **Other storage types**: 1-2 hours each
- **Documentation**: 1 hour

**Total: ~1-2 days of work**

Much simpler and more maintainable! 🚀
