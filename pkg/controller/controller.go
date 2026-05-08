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

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/springfield-csi/driver/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	nodeNameTopologyKey = "hostname"
	defaultVolumeSize   = "3GiB"
	maxNameLength       = 20

	// Volume group names for different storage types
	lvmVolumeGroup = "csi-vg-lvm"
	stratisPoolName = "csi-pool-stratis"
)

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeLVM     StorageType = "lvm"
	StorageTypeMD      StorageType = "mdraid"
	StorageTypeBTRFS   StorageType = "btrfs"
	StorageTypeStratis StorageType = "stratis"
)

// VolumeInfo stores information about a created volume
type VolumeInfo struct {
	Name          string
	CSIVolumeID   string
	BlockPath     string
	CapacityBytes int64
	Disks         []string
	StorageType   StorageType
	FSType        string
	// Storage-specific metadata
	VolumeGroup string // for LVM
	PoolName    string // for Stratis
}

// Service implements the CSI Controller service
type Service struct {
	csi.UnimplementedControllerServer
	nodeID       string
	lvmMgr       *storage.LVMManager
	mdMgr        *storage.MDManager
	btrfsMgr     *storage.BTRFSManager
	stratisMgr   *storage.StratisManager
	volumes      map[string]*VolumeInfo
	volumesMu    sync.RWMutex
	vgInitialized map[string]bool // Track initialized volume groups
	vgMu         sync.Mutex
}

// New creates a new Controller service
func New(nodeID string) *Service {
	executor := storage.NewExecutor()

	return &Service{
		nodeID:        nodeID,
		lvmMgr:        storage.NewLVMManager(executor),
		mdMgr:         storage.NewMDManager(executor),
		btrfsMgr:      storage.NewBTRFSManager(executor),
		stratisMgr:    storage.NewStratisManager(executor),
		volumes:       make(map[string]*VolumeInfo),
		vgInitialized: make(map[string]bool),
	}
}

// Setup initializes the controller (no-op now, kept for compatibility)
func (s *Service) Setup() error {
	klog.Info("Controller setup complete (no initialization required)")
	return nil
}

// getVolume retrieves a volume by ID
func (s *Service) getVolume(volumeID string) *VolumeInfo {
	s.volumesMu.RLock()
	defer s.volumesMu.RUnlock()
	return s.volumes[volumeID]
}

// addVolume adds a volume to the internal map
func (s *Service) addVolume(vol *VolumeInfo) {
	s.volumesMu.Lock()
	defer s.volumesMu.Unlock()
	s.volumes[vol.CSIVolumeID] = vol
}

// removeVolume removes a volume from the internal map
func (s *Service) removeVolume(volumeID string) {
	s.volumesMu.Lock()
	defer s.volumesMu.Unlock()
	delete(s.volumes, volumeID)
}

// shortenName truncates long volume names to avoid issues with LVM and MD
func shortenName(name string) string {
	if len(name) <= maxNameLength {
		return name
	}
	return name[len(name)-maxNameLength:]
}

// parseStorageType converts a string to a StorageType
func parseStorageType(typeStr string) StorageType {
	switch strings.ToLower(typeStr) {
	case "lvm", "device_type_lvm":
		return StorageTypeLVM
	case "mdraid", "md", "device_type_md":
		return StorageTypeMD
	case "stratis", "device_type_stratis":
		return StorageTypeStratis
	case "btrfs", "device_type_btrfs":
		return StorageTypeBTRFS
	default:
		return StorageTypeLVM // Default to LVM
	}
}

// ensureVolumeGroup ensures an LVM volume group exists for the given devices
func (s *Service) ensureVolumeGroup(vgName string, devices []string) error {
	s.vgMu.Lock()
	defer s.vgMu.Unlock()

	if s.vgInitialized[vgName] {
		return nil
	}

	// Check if VG already exists
	exists, err := s.lvmMgr.VolumeGroupExists(vgName)
	if err != nil {
		return fmt.Errorf("failed to check VG existence: %w", err)
	}

	if !exists {
		klog.Infof("Creating volume group %s with devices %v", vgName, devices)
		if err := s.lvmMgr.CreateVolumeGroup(vgName, devices); err != nil {
			return fmt.Errorf("failed to create VG: %w", err)
		}
	}

	s.vgInitialized[vgName] = true
	return nil
}

// ensureStratisPool ensures a Stratis pool exists
func (s *Service) ensureStratisPool(poolName string, devices []string) error {
	s.vgMu.Lock()
	defer s.vgMu.Unlock()

	if s.vgInitialized[poolName] {
		return nil
	}

	if err := s.stratisMgr.CreatePool(poolName, devices); err != nil {
		return fmt.Errorf("failed to ensure Stratis pool: %w", err)
	}

	s.vgInitialized[poolName] = true
	return nil
}

// CreateVolume creates a new volume
func (s *Service) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.Infof("CreateVolume: %s", req.GetName())

	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name is required")
	}

	// Check if volume already exists
	if vol := s.getVolume(req.GetName()); vol != nil {
		reqSize := req.GetCapacityRange().GetRequiredBytes()
		limitSize := req.GetCapacityRange().GetLimitBytes()

		if vol.CapacityBytes == reqSize || vol.CapacityBytes == limitSize {
			klog.V(4).Infof("Volume %s already exists with matching capacity", req.GetName())
			return &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      vol.CSIVolumeID,
					CapacityBytes: vol.CapacityBytes,
					VolumeContext: map[string]string{
						"csi_name":   vol.Name,
						"fs_type":    vol.FSType,
						"block_path": vol.BlockPath,
					},
					AccessibleTopology: []*csi.Topology{
						{Segments: map[string]string{nodeNameTopologyKey: s.nodeID}},
					},
				},
			}, nil
		}
		return nil, status.Error(codes.AlreadyExists, "Volume already exists with different capacity")
	}

	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "At least one volume capability is required")
	}

	// Determine size
	size := req.GetCapacityRange().GetRequiredBytes()
	if size == 0 {
		size = req.GetCapacityRange().GetLimitBytes()
	}
	if size == 0 {
		size = 10485760 // Default 10MiB
	}

	// Parse parameters
	var disks []string
	storageType := StorageTypeLVM
	fsType := "xfs"

	for k, v := range req.GetParameters() {
		klog.V(4).Infof("Parameter %s: %s", k, v)
		switch k {
		case "disks", "devices":
			disks = strings.Split(v, ",")
		case "storagetype", "blivettype", "type":
			storageType = parseStorageType(v)
		case "fstype":
			fsType = v
		}
	}

	if len(disks) == 0 {
		return nil, status.Error(codes.InvalidArgument, "No disks specified in parameters")
	}

	// Validate volume capabilities
	for _, cap := range req.GetVolumeCapabilities() {
		if cap.GetMount() != nil {
			if cap.GetMount().GetFsType() != "" {
				fsType = cap.GetMount().GetFsType()
			}

			// Validate filesystem type
			if fsType != "xfs" && fsType != "btrfs" && fsType != "ext4" {
				return nil, status.Errorf(codes.InvalidArgument, "Unsupported filesystem type: %s", fsType)
			}
		}

		// Only support SINGLE_NODE_WRITER
		if cap.GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			return nil, status.Error(codes.InvalidArgument, "Only SINGLE_NODE_WRITER access mode is supported")
		}
	}

	// Get node name from topology
	nodeName := s.nodeID
	if req.GetAccessibilityRequirements() != nil &&
		len(req.GetAccessibilityRequirements().GetPreferred()) > 0 {
		if segments := req.GetAccessibilityRequirements().GetPreferred()[0].GetSegments(); segments != nil {
			if name, ok := segments[nodeNameTopologyKey]; ok {
				nodeName = name
			}
		}
	}

	// Shorten name if necessary
	shortName := shortenName(req.GetName())
	klog.V(4).Infof("Creating volume: original=%s, short=%s, type=%s", req.GetName(), shortName, storageType)

	// Create the volume based on storage type
	var devicePath string
	var err error
	volInfo := &VolumeInfo{
		Name:          req.GetName(),
		CSIVolumeID:   req.GetName(),
		CapacityBytes: size,
		Disks:         disks,
		StorageType:   storageType,
		FSType:        fsType,
	}

	switch storageType {
	case StorageTypeLVM:
		// Ensure volume group exists
		if err := s.ensureVolumeGroup(lvmVolumeGroup, disks); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to ensure VG: %v", err)
		}

		// Create logical volume
		devicePath, err = s.lvmMgr.CreateLogicalVolume(lvmVolumeGroup, shortName, defaultVolumeSize)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to create LV: %v", err)
		}

		// Create filesystem
		if err := s.lvmMgr.CreateFilesystem(devicePath, fsType); err != nil {
			// Cleanup on failure
			s.lvmMgr.DeleteLogicalVolume(lvmVolumeGroup, shortName)
			return nil, status.Errorf(codes.Internal, "Failed to create filesystem: %v", err)
		}

		volInfo.VolumeGroup = lvmVolumeGroup

	case StorageTypeMD:
		// Create MD RAID array
		devicePath, err = s.mdMgr.CreateArray(shortName, disks, "raid1")
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to create MD array: %v", err)
		}

		// Create filesystem
		if err := s.mdMgr.CreateFilesystem(devicePath, fsType); err != nil {
			s.mdMgr.DeleteArray(shortName)
			return nil, status.Errorf(codes.Internal, "Failed to create filesystem: %v", err)
		}

	case StorageTypeBTRFS:
		// Create BTRFS filesystem
		devicePath, err = s.btrfsMgr.CreateFilesystem(shortName, disks, "raid0", "raid0")
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to create BTRFS: %v", err)
		}

	case StorageTypeStratis:
		// Ensure Stratis pool exists
		if err := s.ensureStratisPool(stratisPoolName, disks); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to ensure Stratis pool: %v", err)
		}

		// Create Stratis filesystem
		devicePath, err = s.stratisMgr.CreateFilesystem(stratisPoolName, shortName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to create Stratis FS: %v", err)
		}

		volInfo.PoolName = stratisPoolName

	default:
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported storage type: %s", storageType)
	}

	volInfo.BlockPath = devicePath
	s.addVolume(volInfo)

	klog.Infof("Created volume %s at %s (type: %s)", req.GetName(), devicePath, storageType)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      req.GetName(),
			CapacityBytes: size,
			VolumeContext: map[string]string{
				"csi_name":   req.GetName(),
				"fs_type":    fsType,
				"block_path": devicePath,
			},
			AccessibleTopology: []*csi.Topology{
				{Segments: map[string]string{nodeNameTopologyKey: nodeName}},
			},
		},
	}, nil
}

// DeleteVolume deletes a volume
func (s *Service) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.Infof("DeleteVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	vol := s.getVolume(req.GetVolumeId())
	if vol == nil {
		klog.V(4).Infof("Volume %s not found, already deleted", req.GetVolumeId())
		return &csi.DeleteVolumeResponse{}, nil
	}

	shortName := shortenName(vol.Name)

	// Delete based on storage type
	var err error
	switch vol.StorageType {
	case StorageTypeLVM:
		err = s.lvmMgr.DeleteLogicalVolume(vol.VolumeGroup, shortName)

	case StorageTypeMD:
		err = s.mdMgr.DeleteArray(shortName)

	case StorageTypeBTRFS:
		// For BTRFS, we just wipe the first device
		if len(vol.Disks) > 0 {
			err = s.btrfsMgr.DeleteFilesystem(vol.Disks[0])
		}

	case StorageTypeStratis:
		err = s.stratisMgr.DeleteFilesystem(vol.PoolName, shortName)

	default:
		err = fmt.Errorf("unsupported storage type: %s", vol.StorageType)
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete volume: %v", err)
	}

	s.removeVolume(req.GetVolumeId())
	klog.Infof("Deleted volume %s", req.GetVolumeId())

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches a volume to a node
func (s *Service) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	klog.Infof("ControllerPublishVolume: %s to node %s", req.GetVolumeId(), req.GetNodeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Node ID is required")
	}

	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability is required")
	}

	vol := s.getVolume(req.GetVolumeId())
	if vol == nil {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"block_path": vol.BlockPath,
		},
	}, nil
}

// ControllerUnpublishVolume detaches a volume from a node
func (s *Service) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	klog.Infof("ControllerUnpublishVolume: %s from node %s", req.GetVolumeId(), req.GetNodeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities validates volume capabilities
func (s *Service) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "At least one volume capability is required")
	}

	vol := s.getVolume(req.GetVolumeId())
	if vol == nil {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	// Validate each capability
	for _, cap := range req.GetVolumeCapabilities() {
		if cap.GetMount() != nil {
			fsType := cap.GetMount().GetFsType()
			if fsType != "" && fsType != "xfs" && fsType != "btrfs" && fsType != "ext4" {
				return &csi.ValidateVolumeCapabilitiesResponse{
					Message: fmt.Sprintf("Unsupported filesystem type: %s", fsType),
				}, nil
			}
		}

		if cap.GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Message: "Only SINGLE_NODE_WRITER access mode is supported",
			}, nil
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

// ListVolumes lists all volumes
func (s *Service) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Info("ListVolumes")

	s.volumesMu.RLock()
	defer s.volumesMu.RUnlock()

	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range s.volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.CSIVolumeID,
				CapacityBytes: vol.CapacityBytes,
			},
		})
	}

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// GetCapacity returns available capacity
func (s *Service) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Info("GetCapacity")

	// Try to get actual capacity from LVM
	total, free, err := s.lvmMgr.GetVolumeGroupSize(lvmVolumeGroup)
	if err != nil {
		klog.V(4).Infof("Could not get VG size: %v, returning default", err)
		return &csi.GetCapacityResponse{
			AvailableCapacity: 100 * 1024 * 1024 * 1024, // 100GB placeholder
		}, nil
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: free,
		MaximumVolumeSize: &csi.CapacityRange{RequiredBytes: total},
	}, nil
}

// ControllerGetCapabilities returns controller capabilities
func (s *Service) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Info("ControllerGetCapabilities")

	capabilities := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_READONLY,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
		csi.ControllerServiceCapability_RPC_GET_VOLUME,
	}

	var csiCaps []*csi.ControllerServiceCapability
	for _, cap := range capabilities {
		csiCaps = append(csiCaps, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

// CreateSnapshot creates a snapshot (not implemented)
func (s *Service) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot not implemented")
}

// DeleteSnapshot deletes a snapshot (not implemented)
func (s *Service) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot not implemented")
}

// ListSnapshots lists snapshots (not implemented)
func (s *Service) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ListSnapshots not implemented")
}

// ControllerExpandVolume expands a volume
func (s *Service) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	klog.V(4).Infof("ControllerExpandVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	vol := s.getVolume(req.GetVolumeId())
	if vol == nil {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	// Only LVM supports online expansion currently
	if vol.StorageType == StorageTypeLVM {
		newSize := fmt.Sprintf("%dB", req.GetCapacityRange().GetRequiredBytes())
		shortName := shortenName(vol.Name)
		if err := s.lvmMgr.ResizeVolume(vol.VolumeGroup, shortName, newSize); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to resize volume: %v", err)
		}

		vol.CapacityBytes = req.GetCapacityRange().GetRequiredBytes()
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         vol.CapacityBytes,
		NodeExpansionRequired: false,
	}, nil
}

// ControllerGetVolume gets volume information
func (s *Service) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	klog.V(4).Infof("ControllerGetVolume: %s", req.GetVolumeId())

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	vol := s.getVolume(req.GetVolumeId())
	if vol == nil {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.CSIVolumeID,
			CapacityBytes: vol.CapacityBytes,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: false,
				Message:  "Volume is healthy",
			},
		},
	}, nil
}
