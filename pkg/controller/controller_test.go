// Copyright (C) 2023-2026  Red Hat, Inc.
//
// Unit tests for controller service

package controller

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
			want:  "bd01-1d12d1313396",
		},
		{
			name:  "exactly max length",
			input: "12345678901234567890",
			want:  "12345678901234567890",
		},
		{
			name:  "one over max length",
			input: "123456789012345678901",
			want:  "23456789012345678901",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenName(tt.input)
			if got != tt.want {
				t.Errorf("shortenName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) > maxNameLength {
				t.Errorf("shortenName(%q) returned name longer than max: %d > %d",
					tt.input, len(got), maxNameLength)
			}
		})
	}
}

func TestParseStorageType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  StorageType
	}{
		{
			name:  "lvm lowercase",
			input: "lvm",
			want:  StorageTypeLVM,
		},
		{
			name:  "LVM uppercase",
			input: "LVM",
			want:  StorageTypeLVM,
		},
		{
			name:  "device_type_lvm",
			input: "DEVICE_TYPE_LVM",
			want:  StorageTypeLVM,
		},
		{
			name:  "mdraid",
			input: "mdraid",
			want:  StorageTypeMD,
		},
		{
			name:  "md",
			input: "md",
			want:  StorageTypeMD,
		},
		{
			name:  "btrfs",
			input: "btrfs",
			want:  StorageTypeBTRFS,
		},
		{
			name:  "stratis",
			input: "stratis",
			want:  StorageTypeStratis,
		},
		{
			name:  "unknown defaults to lvm",
			input: "unknown",
			want:  StorageTypeLVM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStorageType(tt.input)
			if got != tt.want {
				t.Errorf("parseStorageType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

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
			name: "empty capabilities",
			req: &csi.CreateVolumeRequest{
				Name:               "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid access mode",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
				Parameters: map[string]string{
					"disks": "/dev/loop0,/dev/loop1",
				},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing disks parameter",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
				Parameters: map[string]string{},
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
				t.Errorf("expected code %v, got %v (message: %s)",
					tt.wantCode, st.Code(), st.Message())
			}
		})
	}
}

func TestDeleteVolume_Validation(t *testing.T) {
	ctrl := New("test-node")

	tests := []struct {
		name     string
		req      *csi.DeleteVolumeRequest
		wantCode codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "",
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctrl.DeleteVolume(context.Background(), tt.req)

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

func TestDeleteVolume_NotFound(t *testing.T) {
	ctrl := New("test-node")

	req := &csi.DeleteVolumeRequest{
		VolumeId: "non-existent-volume",
	}

	resp, err := ctrl.DeleteVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("DeleteVolume() error = %v, want nil (idempotent)", err)
	}

	if resp == nil {
		t.Error("expected response, got nil")
	}
}

func TestControllerPublishVolume_Validation(t *testing.T) {
	ctrl := New("test-node")

	tests := []struct {
		name     string
		req      *csi.ControllerPublishVolumeRequest
		wantCode codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "",
				NodeId:   "test-node",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing node ID",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "test-volume",
				NodeId:   "",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing volume capability",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId:         "test-volume",
				NodeId:           "test-node",
				VolumeCapability: nil,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "volume not found",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "non-existent",
				NodeId:   "test-node",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
			wantCode: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctrl.ControllerPublishVolume(context.Background(), tt.req)

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

func TestValidateVolumeCapabilities_Validation(t *testing.T) {
	ctrl := New("test-node")

	tests := []struct {
		name     string
		req      *csi.ValidateVolumeCapabilitiesRequest
		wantCode codes.Code
	}{
		{
			name: "missing volume ID",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: "",
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing capabilities",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId:           "test-volume",
				VolumeCapabilities: nil,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "volume not found",
			req: &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId: "non-existent",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			wantCode: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctrl.ValidateVolumeCapabilities(context.Background(), tt.req)

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
