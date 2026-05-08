// Copyright (C) 2023-2026  Red Hat, Inc.
//
// Unit tests for LVM manager

package storage

import (
	"fmt"
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
		wantArgs  []string
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
			wantArgs: []string{"-n", "test-lv", "-L", "1G", "test-vg"},
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
			if tt.wantArgs != nil {
				if !mockExec.AssertCommandCalled("lvcreate", tt.wantArgs...) {
					t.Errorf("lvcreate not called with expected args: %v", tt.wantArgs)
				}
			}
		})
	}
}

func TestDeleteLogicalVolume(t *testing.T) {
	tests := []struct {
		name     string
		vgName   string
		lvName   string
		mockResp testutil.Response
		wantErr  bool
	}{
		{
			name:   "successful deletion",
			vgName: "test-vg",
			lvName: "test-lv",
			mockResp: testutil.Response{
				Stdout: "",
				Err:    nil,
			},
			wantErr: false,
		},
		{
			name:   "lvremove fails",
			vgName: "test-vg",
			lvName: "test-lv",
			mockResp: testutil.Response{
				Stderr: "Failed to find logical volume",
				Err:    fmt.Errorf("exit status 5"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockExecutor()
			mockExec.AddResponse("lvremove", tt.mockResp)

			mgr := &LVMManager{executor: mockExec}

			err := mgr.DeleteLogicalVolume(tt.vgName, tt.lvName)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteLogicalVolume() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify lvremove was called
			if mockExec.CommandCallCount("lvremove") != 1 {
				t.Error("lvremove should be called exactly once")
			}
		})
	}
}

func TestCreateFilesystem(t *testing.T) {
	tests := []struct {
		name       string
		device     string
		fsType     string
		mockResp   testutil.Response
		wantCmd    string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "create xfs filesystem",
			device: "/dev/test-vg/test-lv",
			fsType: "xfs",
			mockResp: testutil.Response{
				Stdout: "meta-data=/dev/test-vg/test-lv...",
				Err:    nil,
			},
			wantCmd: "mkfs.xfs",
			wantErr: false,
		},
		{
			name:   "create ext4 filesystem",
			device: "/dev/test-vg/test-lv",
			fsType: "ext4",
			mockResp: testutil.Response{
				Stdout: "mke2fs 1.46.5...",
				Err:    nil,
			},
			wantCmd: "mkfs.ext4",
			wantErr: false,
		},
		{
			name:       "unsupported filesystem",
			device:     "/dev/test-vg/test-lv",
			fsType:     "ntfs",
			wantErr:    true,
			wantErrMsg: "unsupported filesystem type",
		},
		{
			name:   "mkfs fails",
			device: "/dev/test-vg/test-lv",
			fsType: "xfs",
			mockResp: testutil.Response{
				Stderr: "device is busy",
				Err:    fmt.Errorf("exit status 1"),
			},
			wantCmd: "mkfs.xfs",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockExecutor()
			if tt.wantCmd != "" {
				mockExec.AddResponse(tt.wantCmd, tt.mockResp)
			}

			mgr := &LVMManager{executor: mockExec}

			err := mgr.CreateFilesystem(tt.device, tt.fsType)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateFilesystem() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErrMsg != "" && err != nil {
				if !contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error message = %v, want to contain %v", err.Error(), tt.wantErrMsg)
				}
			}

			// Verify correct mkfs command was called
			if tt.wantCmd != "" && !tt.wantErr {
				if mockExec.CommandCallCount(tt.wantCmd) != 1 {
					t.Errorf("%s should be called exactly once", tt.wantCmd)
				}
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
				Stderr: "Volume group \"missing-vg\" not found",
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

func TestVolumeExists(t *testing.T) {
	tests := []struct {
		name     string
		vgName   string
		lvName   string
		mockResp testutil.Response
		want     bool
		wantErr  bool
	}{
		{
			name:   "volume exists",
			vgName: "test-vg",
			lvName: "test-lv",
			mockResp: testutil.Response{
				Stdout: "  test-vg/test-lv\n",
				Err:    nil,
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "volume does not exist",
			vgName: "test-vg",
			lvName: "missing-lv",
			mockResp: testutil.Response{
				Stderr: "Failed to find logical volume",
				Err:    fmt.Errorf("exit status 5"),
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockExecutor()
			mockExec.AddResponse("lvs", tt.mockResp)

			mgr := &LVMManager{executor: mockExec}

			got, err := mgr.VolumeExists(tt.vgName, tt.lvName)

			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("VolumeExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetVolumeGroupSize(t *testing.T) {
	tests := []struct {
		name      string
		vgName    string
		mockResp  testutil.Response
		wantTotal int64
		wantFree  int64
		wantErr   bool
	}{
		{
			name:   "successful query",
			vgName: "test-vg",
			mockResp: testutil.Response{
				Stdout: "  107374182400  53687091200\n", // 100GB total, 50GB free
				Err:    nil,
			},
			wantTotal: 107374182400,
			wantFree:  53687091200,
			wantErr:   false,
		},
		{
			name:   "vgs command fails",
			vgName: "test-vg",
			mockResp: testutil.Response{
				Stderr: "Volume group not found",
				Err:    fmt.Errorf("exit status 5"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := testutil.NewMockExecutor()
			mockExec.AddResponse("vgs", tt.mockResp)

			mgr := &LVMManager{executor: mockExec}

			total, free, err := mgr.GetVolumeGroupSize(tt.vgName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetVolumeGroupSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if total != tt.wantTotal {
					t.Errorf("GetVolumeGroupSize() total = %v, want %v", total, tt.wantTotal)
				}
				if free != tt.wantFree {
					t.Errorf("GetVolumeGroupSize() free = %v, want %v", free, tt.wantFree)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 len(s) > len(substr) && s[1:len(s)-1] != s &&
		 (s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}
