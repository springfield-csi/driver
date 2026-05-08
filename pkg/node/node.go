// Copyright (C) 2023-2026  Red Hat, Inc.
//
// This copyrighted material is made available to anyone wishing to use,
// modify, copy, or redistribute it subject to the terms and conditions of
// the GNU General Public License v.2, or (at your option) any later version.
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY expressed or implied, including the implied warranties of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
// Public License for more details.  You should have received a copy of the
// GNU General Public License along with this program; if not, write to the
// Free Software Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
// 02110-1301, USA.  Any Red Hat trademarks that are incorporated in the
// source code or documentation are not subject to the GNU General Public
// License and may only be used or replicated with the express permission of
// Red Hat, Inc.
//
// Red Hat Author(s): Todd Gill <tgill@redhat.com>

package node

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

// Service implements the CSI Node service
type Service struct {
	csi.UnimplementedNodeServer
	nodeID string
	mounter mount.Interface
}

// New creates a new Node service
func New(nodeID string) *Service {
	return &Service{
		nodeID:  nodeID,
		mounter: mount.New(""),
	}
}

// NodeStageVolume mounts the volume to a staging path
func (s *Service) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof("NodeStageVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target path is required")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability is required")
	}

	stagingPath := req.GetStagingTargetPath()

	// Get block device path from publish context
	blockPath, ok := req.GetPublishContext()["block_path"]
	if !ok || blockPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Block path not found in publish context")
	}

	// Replace /dev with /hostdev for container environments
	devPath := strings.Replace(blockPath, "/dev", "/hostdev", 1)
	klog.V(4).Infof("Device path: %s -> %s", blockPath, devPath)

	// Create staging directory if it doesn't exist
	if err := os.MkdirAll(stagingPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create staging directory: %v", err)
	}

	// Check if already mounted
	notMnt, err := s.mounter.IsLikelyNotMountPoint(stagingPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	if !notMnt {
		klog.V(4).Infof("Volume %s already staged at %s", req.GetVolumeId(), stagingPath)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	// Get filesystem type from volume capability
	fsType := "xfs"
	if req.GetVolumeCapability().GetMount() != nil {
		if req.GetVolumeCapability().GetMount().GetFsType() != "" {
			fsType = req.GetVolumeCapability().GetMount().GetFsType()
		}
	}

	// Mount options
	options := []string{}
	if req.GetVolumeCapability().GetMount() != nil {
		options = req.GetVolumeCapability().GetMount().GetMountFlags()
	}

	// Perform the mount
	klog.Infof("Mounting %s to %s with fstype %s", devPath, stagingPath, fsType)
	if err := s.mounter.Mount(devPath, stagingPath, fsType, options); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to mount volume: %v", err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume unmounts the volume from a staging path
func (s *Service) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.Infof("NodeUnstageVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target path is required")
	}

	stagingPath := req.GetStagingTargetPath()

	// Check if the path exists
	if _, err := os.Stat(stagingPath); os.IsNotExist(err) {
		klog.V(4).Infof("Staging path %s does not exist, already unstaged", stagingPath)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	// Check if it's a mount point
	notMnt, err := s.mounter.IsLikelyNotMountPoint(stagingPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	if notMnt {
		klog.V(4).Infof("Staging path %s is not mounted", stagingPath)
	} else {
		// Unmount
		klog.Infof("Unmounting %s", stagingPath)
		if err := s.mounter.Unmount(stagingPath); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to unmount volume: %v", err)
		}
	}

	// Remove the staging directory
	if err := os.Remove(stagingPath); err != nil && !os.IsNotExist(err) {
		klog.Warningf("Failed to remove staging directory %s: %v", stagingPath, err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume bind mounts the volume from staging to target path
func (s *Service) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof("NodePublishVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path is required")
	}

	if req.GetStagingTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Staging target path is required")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability is required")
	}

	stagingPath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()

	// Validate access mode
	if req.GetVolumeCapability().GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		return nil, status.Error(codes.InvalidArgument, "Only SINGLE_NODE_WRITER access mode is supported")
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create target directory: %v", err)
	}

	// Check if already mounted
	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	if !notMnt {
		klog.V(4).Infof("Volume %s already published at %s", req.GetVolumeId(), targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Bind mount from staging to target
	options := []string{"bind"}
	if req.GetReadonly() {
		options = append(options, "ro")
	}

	klog.Infof("Bind mounting %s to %s", stagingPath, targetPath)
	if err := s.mounter.Mount(stagingPath, targetPath, "", options); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to bind mount volume: %v", err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts the volume from the target path
func (s *Service) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof("NodeUnpublishVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path is required")
	}

	targetPath := req.GetTargetPath()

	// Check if the path exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		klog.V(4).Infof("Target path %s does not exist, already unpublished", targetPath)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// Check if it's a mount point
	notMnt, err := s.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	if notMnt {
		klog.V(4).Infof("Target path %s is not mounted", targetPath)
	} else {
		// Unmount
		klog.Infof("Unmounting %s", targetPath)
		if err := s.mounter.Unmount(targetPath); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to unmount volume: %v", err)
		}
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities returns node capabilities
func (s *Service) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Info("NodeGetCapabilities")

	capabilities := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}

	var csiCaps []*csi.NodeServiceCapability
	for _, cap := range capabilities {
		csiCaps = append(csiCaps, &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

// NodeGetInfo returns node information
func (s *Service) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: nodeID=%s", s.nodeID)

	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeID,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				"hostname": s.nodeID,
			},
		},
	}, nil
}

// NodeExpandVolume expands a node volume
func (s *Service) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	klog.V(4).Infof("NodeExpandVolume: %s", req.GetVolumeId())

	// TODO: Implement actual filesystem expansion
	return &csi.NodeExpandVolumeResponse{}, nil
}

// NodeGetVolumeStats returns volume statistics
func (s *Service) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	klog.V(4).Infof("NodeGetVolumeStats: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetVolumePath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume path is required")
	}

	volumePath := req.GetVolumePath()

	// Check if path exists and is a mount point
	notMnt, err := s.mounter.IsLikelyNotMountPoint(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "Volume path does not exist")
		}
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	if notMnt {
		return nil, status.Error(codes.NotFound, "Volume is not mounted")
	}

	// Get filesystem stats
	available, capacity, used, inodesFree, inodes, inodesUsed, err := getFsStats(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get filesystem stats: %v", err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: available,
				Total:     capacity,
				Used:      used,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: inodesFree,
				Total:     inodes,
				Used:      inodesUsed,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: false,
			Message:  "Volume is healthy",
		},
	}, nil
}

// getFsStats returns filesystem statistics for a given path
func getFsStats(path string) (available, capacity, used, inodesFree, inodes, inodesUsed int64, err error) {
	// Use k8s.io/mount-utils for consistent stats gathering
	statfs := &mount.Statfs{}
	mounter := mount.New("")

	info, err := mounter.GetMountRefs(path)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to get mount refs: %w", err)
	}

	if len(info) == 0 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("no mount information found for %s", path)
	}

	// Get actual filesystem stats using syscall
	stats, err := statfs.StatFS(path)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to stat filesystem: %w", err)
	}

	available = stats.Available
	capacity = stats.Capacity
	used = capacity - available
	inodesFree = stats.InodesFree
	inodes = stats.Inodes
	inodesUsed = inodes - inodesFree

	return available, capacity, used, inodesFree, inodes, inodesUsed, nil
}
