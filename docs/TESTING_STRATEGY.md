# Testing Strategy for Springfield CSI Driver

## Overview

A robust CSI driver requires multiple layers of testing:
1. **Unit Tests** - Test individual functions and methods
2. **Integration Tests** - Test storage managers with mock executors
3. **CSI Sanity Tests** - Validate CSI spec compliance
4. **End-to-End Tests** - Real Kubernetes cluster tests
5. **Storage-Specific Tests** - Verify each backend (LVM, MD, etc.)

## Test Pyramid

```
                    ┌─────────────────┐
                    │   E2E Tests     │  Slowest, most realistic
                    │  (Real K8s)     │
                    └─────────────────┘
                  ┌───────────────────────┐
                  │  CSI Sanity Tests     │  Spec compliance
                  │  (csi-sanity tool)    │
                  └───────────────────────┘
              ┌─────────────────────────────────┐
              │   Integration Tests             │  Storage operations
              │   (Mocked commands)             │
              └─────────────────────────────────┘
          ┌───────────────────────────────────────────┐
          │          Unit Tests                        │  Fastest
          │   (Pure functions, business logic)         │
          └───────────────────────────────────────────┘
```

## 1. Unit Tests

Test individual functions without external dependencies.

### Structure

```
pkg/
├── storage/
│   ├── lvm.go
│   ├── lvm_test.go          # Unit tests for LVM
│   ├── executor.go
│   ├── executor_test.go     # Unit tests for executor
│   └── testutil/
│       └── mock_executor.go # Mock executor for testing
├── controller/
│   ├── controller.go
│   └── controller_test.go   # Unit tests for controller
└── node/
    ├── node.go
    └── node_test.go         # Unit tests for node
```

### Example: Mock Executor

```go
// pkg/storage/testutil/mock_executor.go
package testutil

import "sync"

type MockExecutor struct {
    mu       sync.Mutex
    commands []Command
    responses map[string]Response
}

type Command struct {
    Name string
    Args []string
}

type Response struct {
    Stdout string
    Stderr string
    Err    error
}

func NewMockExecutor() *MockExecutor {
    return &MockExecutor{
        commands:  make([]Command, 0),
        responses: make(map[string]Response),
    }
}

func (m *MockExecutor) AddResponse(cmd string, resp Response) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.responses[cmd] = resp
}

func (m *MockExecutor) Run(name string, args ...string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.commands = append(m.commands, Command{Name: name, Args: args})
    
    key := name
    if resp, ok := m.responses[key]; ok {
        return resp.Stdout, resp.Err
    }
    
    return "", nil
}

func (m *MockExecutor) GetCommands() []Command {
    m.mu.Lock()
    defer m.mu.Unlock()
    return append([]Command{}, m.commands...)
}

func (m *MockExecutor) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.commands = make([]Command, 0)
}
```

### Example: LVM Unit Tests

```go
// pkg/storage/lvm_test.go
package storage

import (
    "testing"
    "github.com/springfield-csi/driver/pkg/storage/testutil"
)

func TestCreateLogicalVolume(t *testing.T) {
    tests := []struct {
        name      string
        vgName    string
        lvName    string
        size      string
        mockResp  testutil.Response
        wantPath  string
        wantErr   bool
    }{
        {
            name:   "successful creation",
            vgName: "test-vg",
            lvName: "test-lv",
            size:   "1G",
            mockResp: testutil.Response{
                Stdout: "",
                Err:    nil,
            },
            wantPath: "/dev/test-vg/test-lv",
            wantErr:  false,
        },
        {
            name:   "lvcreate fails",
            vgName: "test-vg",
            lvName: "test-lv",
            size:   "1G",
            mockResp: testutil.Response{
                Stderr: "Volume group not found",
                Err:    fmt.Errorf("exit status 5"),
            },
            wantPath: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockExec := testutil.NewMockExecutor()
            mockExec.AddResponse("lvcreate", tt.mockResp)
            
            mgr := &LVMManager{executor: mockExec}
            
            gotPath, err := mgr.CreateLogicalVolume(tt.vgName, tt.lvName, tt.size)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateLogicalVolume() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if gotPath != tt.wantPath {
                t.Errorf("CreateLogicalVolume() path = %v, want %v", gotPath, tt.wantPath)
            }
            
            // Verify the command was called correctly
            cmds := mockExec.GetCommands()
            if len(cmds) != 1 {
                t.Fatalf("expected 1 command, got %d", len(cmds))
            }
            
            if cmds[0].Name != "lvcreate" {
                t.Errorf("expected lvcreate command, got %s", cmds[0].Name)
            }
        })
    }
}

func TestVolumeGroupExists(t *testing.T) {
    tests := []struct {
        name     string
        vgName   string
        mockResp testutil.Response
        want     bool
        wantErr  bool
    }{
        {
            name:   "vg exists",
            vgName: "existing-vg",
            mockResp: testutil.Response{
                Stdout: "  VG existing-vg\n",
                Err:    nil,
            },
            want:    true,
            wantErr: false,
        },
        {
            name:   "vg does not exist",
            vgName: "missing-vg",
            mockResp: testutil.Response{
                Stderr: "Volume group not found",
                Err:    fmt.Errorf("exit status 5"),
            },
            want:    false,
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockExec := testutil.NewMockExecutor()
            mockExec.AddResponse("vgs", tt.mockResp)
            
            mgr := &LVMManager{executor: mockExec}
            
            got, err := mgr.VolumeGroupExists(tt.vgName)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("VolumeGroupExists() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if got != tt.want {
                t.Errorf("VolumeGroupExists() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Example: Controller Unit Tests

```go
// pkg/controller/controller_test.go
package controller

import (
    "context"
    "testing"
    
    "github.com/container-storage-interface/spec/lib/go/csi"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func TestCreateVolume_Validation(t *testing.T) {
    ctrl := New("test-node")
    
    tests := []struct {
        name     string
        req      *csi.CreateVolumeRequest
        wantCode codes.Code
    }{
        {
            name: "missing volume name",
            req: &csi.CreateVolumeRequest{
                Name: "",
            },
            wantCode: codes.InvalidArgument,
        },
        {
            name: "missing capabilities",
            req: &csi.CreateVolumeRequest{
                Name:               "test-volume",
                VolumeCapabilities: nil,
            },
            wantCode: codes.InvalidArgument,
        },
        {
            name: "invalid access mode",
            req: &csi.CreateVolumeRequest{
                Name: "test-volume",
                VolumeCapabilities: []*csi.VolumeCapability{
                    {
                        AccessMode: &csi.VolumeCapability_AccessMode{
                            Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
                        },
                    },
                },
            },
            wantCode: codes.InvalidArgument,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ctrl.CreateVolume(context.Background(), tt.req)
            
            if err == nil {
                t.Fatal("expected error, got nil")
            }
            
            st, ok := status.FromError(err)
            if !ok {
                t.Fatalf("expected gRPC status error, got %v", err)
            }
            
            if st.Code() != tt.wantCode {
                t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
            }
        })
    }
}

func TestShortenName(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {
            name:  "short name unchanged",
            input: "short",
            want:  "short",
        },
        {
            name:  "long PVC name truncated",
            input: "pvc-d5343444-7614-48bf-bd01-1d12d1313396",
            want:  "d01-1d12d1313396",
        },
        {
            name:  "exactly max length",
            input: "12345678901234567890",
            want:  "12345678901234567890",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := shortenName(tt.input)
            if got != tt.want {
                t.Errorf("shortenName(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## 2. Integration Tests

Test storage managers with actual command execution in a safe environment.

### Structure

```
test/
├── integration/
│   ├── setup_test.go        # Test setup/teardown
│   ├── lvm_test.go          # LVM integration tests
│   ├── mdraid_test.go       # MD RAID integration tests
│   └── btrfs_test.go        # BTRFS integration tests
└── fixtures/
    ├── loop_devices.sh      # Create loop devices for testing
    └── cleanup.sh           # Clean up test resources
```

### Example: LVM Integration Test

```go
// test/integration/lvm_test.go
// +build integration

package integration

import (
    "os"
    "testing"
    
    "github.com/springfield-csi/driver/pkg/storage"
)

func TestLVMIntegration(t *testing.T) {
    if os.Getuid() != 0 {
        t.Skip("integration tests require root")
    }
    
    // Setup: create loop devices
    devices := setupLoopDevices(t, 2, "1G")
    defer cleanupLoopDevices(t, devices)
    
    executor := storage.NewExecutor()
    mgr := storage.NewLVMManager(executor)
    
    vgName := "test-vg-integration"
    lvName := "test-lv"
    
    // Test: Create VG
    t.Run("CreateVolumeGroup", func(t *testing.T) {
        err := mgr.CreateVolumeGroup(vgName, devices)
        if err != nil {
            t.Fatalf("CreateVolumeGroup failed: %v", err)
        }
        
        exists, err := mgr.VolumeGroupExists(vgName)
        if err != nil {
            t.Fatalf("VolumeGroupExists failed: %v", err)
        }
        if !exists {
            t.Error("VG should exist after creation")
        }
    })
    
    // Test: Create LV
    var devicePath string
    t.Run("CreateLogicalVolume", func(t *testing.T) {
        var err error
        devicePath, err = mgr.CreateLogicalVolume(vgName, lvName, "500M")
        if err != nil {
            t.Fatalf("CreateLogicalVolume failed: %v", err)
        }
        
        if devicePath != "/dev/"+vgName+"/"+lvName {
            t.Errorf("unexpected device path: %s", devicePath)
        }
        
        exists, err := mgr.VolumeExists(vgName, lvName)
        if err != nil {
            t.Fatalf("VolumeExists failed: %v", err)
        }
        if !exists {
            t.Error("LV should exist after creation")
        }
    })
    
    // Test: Create filesystem
    t.Run("CreateFilesystem", func(t *testing.T) {
        err := mgr.CreateFilesystem(devicePath, "xfs")
        if err != nil {
            t.Fatalf("CreateFilesystem failed: %v", err)
        }
    })
    
    // Test: Delete LV
    t.Run("DeleteLogicalVolume", func(t *testing.T) {
        err := mgr.DeleteLogicalVolume(vgName, lvName)
        if err != nil {
            t.Fatalf("DeleteLogicalVolume failed: %v", err)
        }
        
        exists, _ := mgr.VolumeExists(vgName, lvName)
        if exists {
            t.Error("LV should not exist after deletion")
        }
    })
}

func setupLoopDevices(t *testing.T, count int, size string) []string {
    // Create loop devices for testing
    // Implementation details...
}

func cleanupLoopDevices(t *testing.T, devices []string) {
    // Clean up loop devices
    // Implementation details...
}
```

## 3. CSI Sanity Tests

Use the official `csi-sanity` tool to verify CSI spec compliance.

### Installation

```bash
go install github.com/kubernetes-csi/csi-test/v5/cmd/csi-sanity@latest
```

### Running CSI Sanity

```bash
# test/sanity/run_sanity.sh
#!/bin/bash

set -e

# Start the CSI driver in the background
./bin/csi-driver \
    --endpoint=unix:///tmp/csi.sock \
    --nodeid=sanity-test-node \
    -v=5 &

DRIVER_PID=$!

# Wait for driver to be ready
sleep 2

# Run sanity tests
csi-sanity \
    --csi.endpoint=unix:///tmp/csi.sock \
    --csi.testnodevolumeattachlimit=true \
    --csi.testvolumesize=1073741824

# Cleanup
kill $DRIVER_PID
rm -f /tmp/csi.sock
```

### Custom Sanity Test Config

```yaml
# test/sanity/config.yaml
sanity:
  endpoint: unix:///tmp/csi.sock
  createStagingPathCmd: mkdir -p
  createTargetPathCmd: mkdir -p
  removeStagingPathCmd: rm -rf
  removeTargetPathCmd: rm -rf
  testVolumeSize: 1073741824  # 1GB
  testVolumeParametersFile: test-params.yaml
```

```yaml
# test/sanity/test-params.yaml
storagetype: lvm
disks: /dev/loop0,/dev/loop1
fstype: xfs
```

## 4. End-to-End Tests

Test the driver in a real Kubernetes cluster.

### Using Kind for E2E Tests

```bash
# test/e2e/setup_kind.sh
#!/bin/bash

set -e

KIND_CLUSTER_NAME="csi-test"

# Create kind cluster
kind create cluster \
    --name $KIND_CLUSTER_NAME \
    --config test/e2e/kind-config.yaml

# Load driver image
kind load docker-image \
    ghcr.io/springfield-csi/driver:test \
    --name $KIND_CLUSTER_NAME

# Deploy driver
kubectl apply -f deploy/helm/springfield-csi/

# Wait for driver to be ready
kubectl wait --for=condition=ready pod \
    -l app=springfield-csi-controller \
    --timeout=120s

echo "E2E environment ready"
```

### E2E Test Framework

```go
// test/e2e/e2e_test.go
// +build e2e

package e2e

import (
    "context"
    "testing"
    "time"
    
    corev1 "k8s.io/api/core/v1"
    storagev1 "k8s.io/api/storage/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
)

type E2ETest struct {
    client    *kubernetes.Clientset
    namespace string
}

func NewE2ETest(t *testing.T) *E2ETest {
    // Initialize Kubernetes client
    // ...
}

func TestVolumeLifecycle(t *testing.T) {
    e2e := NewE2ETest(t)
    
    // Test: Create PVC
    pvc := &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-pvc",
            Namespace: e2e.namespace,
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{
                corev1.ReadWriteOnce,
            },
            StorageClassName: stringPtr("springfield-lvm"),
            Resources: corev1.ResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: resource.MustParse("1Gi"),
                },
            },
        },
    }
    
    _, err := e2e.client.CoreV1().PersistentVolumeClaims(e2e.namespace).Create(
        context.Background(), pvc, metav1.CreateOptions{})
    if err != nil {
        t.Fatalf("Failed to create PVC: %v", err)
    }
    
    // Wait for PVC to be bound
    err = e2e.waitForPVCBound("test-pvc", 60*time.Second)
    if err != nil {
        t.Fatalf("PVC not bound: %v", err)
    }
    
    // Test: Create Pod using PVC
    pod := createTestPod("test-pod", "test-pvc")
    _, err = e2e.client.CoreV1().Pods(e2e.namespace).Create(
        context.Background(), pod, metav1.CreateOptions{})
    if err != nil {
        t.Fatalf("Failed to create pod: %v", err)
    }
    
    // Wait for pod to be running
    err = e2e.waitForPodRunning("test-pod", 120*time.Second)
    if err != nil {
        t.Fatalf("Pod not running: %v", err)
    }
    
    // Test: Verify volume is mounted
    mounted, err := e2e.isVolumeMounted("test-pod", "/data")
    if err != nil {
        t.Fatalf("Failed to check mount: %v", err)
    }
    if !mounted {
        t.Error("Volume not mounted in pod")
    }
    
    // Test: Write data to volume
    err = e2e.execInPod("test-pod", []string{
        "sh", "-c", "echo 'test data' > /data/test.txt",
    })
    if err != nil {
        t.Fatalf("Failed to write to volume: %v", err)
    }
    
    // Cleanup
    e2e.cleanup(t)
}
```

## 5. CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Test

on:
  pull_request:
  push:
    branches: [ main ]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Run unit tests
        run: make test
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Install storage tools
        run: |
          sudo apt-get update
          sudo apt-get install -y lvm2 mdadm btrfs-progs xfsprogs
      
      - name: Run integration tests
        run: sudo make test-integration

  csi-sanity:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Build driver
        run: make build
      
      - name: Install csi-sanity
        run: |
          go install github.com/kubernetes-csi/csi-test/v5/cmd/csi-sanity@latest
      
      - name: Run CSI sanity tests
        run: make test-sanity

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Set up Kind
        uses: helm/kind-action@v1.5.0
      
      - name: Build and load image
        run: |
          make image IMAGE_TAG=test
          kind load docker-image ghcr.io/springfield-csi/driver:test
      
      - name: Deploy driver
        run: |
          kubectl apply -f deploy/helm/springfield-csi/
          kubectl wait --for=condition=ready pod -l app=springfield-csi-controller --timeout=120s
      
      - name: Run E2E tests
        run: make test-e2e
```

## 6. Makefile Test Targets

```makefile
# Add to Makefile

.PHONY: test
test:
	@echo "Running unit tests..."
	go test -v -race -cover ./pkg/...

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./test/integration/...

.PHONY: test-sanity
test-sanity:
	@echo "Running CSI sanity tests..."
	./test/sanity/run_sanity.sh

.PHONY: test-e2e
test-e2e:
	@echo "Running E2E tests..."
	go test -v -tags=e2e -timeout=30m ./test/e2e/...

.PHONY: test-all
test-all: test test-integration test-sanity test-e2e
	@echo "All tests passed!"

.PHONY: coverage
coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
```

## Best Practices

### 1. Table-Driven Tests
Always use table-driven tests for multiple test cases:
```go
tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
}{
    // test cases...
}
```

### 2. Test Helpers
Create helper functions to reduce duplication:
```go
func createTestPVC(name, storageClass string) *corev1.PersistentVolumeClaim {
    // ...
}
```

### 3. Cleanup
Always clean up resources:
```go
defer cleanupLoopDevices(t, devices)
defer e2e.cleanup(t)
```

### 4. Skip Conditions
Skip tests when requirements not met:
```go
if os.Getuid() != 0 {
    t.Skip("requires root")
}
```

### 5. Parallel Tests
Run independent tests in parallel:
```go
t.Run("test1", func(t *testing.T) {
    t.Parallel()
    // test...
})
```

## Coverage Goals

- **Unit tests**: > 80% coverage
- **Integration tests**: All storage managers
- **CSI sanity**: 100% passing
- **E2E tests**: Happy path + error cases

## Running Tests Locally

```bash
# Unit tests
make test

# Integration tests (requires root)
sudo make test-integration

# CSI sanity (requires built binary)
make build
make test-sanity

# E2E tests (requires kind cluster)
make test-e2e

# All tests
make test-all
```
