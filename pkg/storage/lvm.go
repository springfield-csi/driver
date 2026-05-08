// Copyright (C) 2023-2026  Red Hat, Inc.
//
// Example of direct LVM management without DBus
// This is a better approach for production CSI drivers

package storage

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

// LVMManager handles LVM operations by executing commands on the host
type LVMManager struct {
	nsenterPath string
	useNsenter  bool
}

// NewLVMManager creates a new LVM manager
func NewLVMManager(useNsenter bool) *LVMManager {
	return &LVMManager{
		nsenterPath: "/usr/bin/nsenter",
		useNsenter:  useNsenter,
	}
}

// execCommand runs a command, optionally using nsenter to run in host namespaces
func (m *LVMManager) execCommand(name string, args ...string) (string, error) {
	var cmd *exec.Cmd

	if m.useNsenter {
		// Run command in host's mount, IPC, and network namespaces
		// This allows the container to manipulate host storage
		nsenterArgs := []string{
			"--mount=/host/proc/1/ns/mnt",
			"--ipc=/host/proc/1/ns/ipc",
			"--net=/host/proc/1/ns/net",
			"--", // End of nsenter args
			name,
		}
		nsenterArgs = append(nsenterArgs, args...)
		cmd = exec.Command(m.nsenterPath, nsenterArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	klog.V(4).Infof("Executing: %s %v", name, args)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %s, stderr: %s, error: %w",
			name, stderr.String(), err)
	}

	return stdout.String(), nil
}

// CreateVolumeGroup creates an LVM volume group
func (m *LVMManager) CreateVolumeGroup(vgName string, devices []string) error {
	klog.Infof("Creating volume group %s on devices %v", vgName, devices)

	// Initialize physical volumes
	for _, dev := range devices {
		if _, err := m.execCommand("pvcreate", "-f", dev); err != nil {
			return fmt.Errorf("failed to create PV on %s: %w", dev, err)
		}
	}

	// Create volume group
	args := append([]string{vgName}, devices...)
	if _, err := m.execCommand("vgcreate", args...); err != nil {
		return fmt.Errorf("failed to create VG %s: %w", vgName, err)
	}

	return nil
}

// CreateLogicalVolume creates an LVM logical volume
func (m *LVMManager) CreateLogicalVolume(vgName, lvName, size string) (string, error) {
	klog.Infof("Creating LV %s/%s with size %s", vgName, lvName, size)

	// Create the LV
	_, err := m.execCommand("lvcreate",
		"-n", lvName,
		"-L", size,
		vgName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create LV: %w", err)
	}

	// Return device path
	devicePath := fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	return devicePath, nil
}

// CreateFilesystem creates a filesystem on a device
func (m *LVMManager) CreateFilesystem(device, fsType string) error {
	klog.Infof("Creating %s filesystem on %s", fsType, device)

	var cmd string
	var args []string

	switch fsType {
	case "xfs":
		cmd = "mkfs.xfs"
		args = []string{"-f", device}
	case "ext4":
		cmd = "mkfs.ext4"
		args = []string{"-F", device}
	case "btrfs":
		cmd = "mkfs.btrfs"
		args = []string{"-f", device}
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	if _, err := m.execCommand(cmd, args...); err != nil {
		return fmt.Errorf("failed to create %s filesystem: %w", fsType, err)
	}

	return nil
}

// DeleteLogicalVolume deletes an LVM logical volume
func (m *LVMManager) DeleteLogicalVolume(vgName, lvName string) error {
	klog.Infof("Deleting LV %s/%s", vgName, lvName)

	lvPath := fmt.Sprintf("%s/%s", vgName, lvName)
	_, err := m.execCommand("lvremove", "-f", lvPath)
	if err != nil {
		return fmt.Errorf("failed to delete LV: %w", err)
	}

	return nil
}

// GetVolumeGroupSize returns the size and free space of a VG
func (m *LVMManager) GetVolumeGroupSize(vgName string) (total, free int64, err error) {
	// Use vgs command with specific format
	output, err := m.execCommand("vgs",
		"--noheadings",
		"--units", "b",
		"--nosuffix",
		"-o", "vg_size,vg_free",
		vgName,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get VG size: %w", err)
	}

	// Parse output: "  total_bytes  free_bytes"
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected vgs output: %s", output)
	}

	fmt.Sscanf(fields[0], "%d", &total)
	fmt.Sscanf(fields[1], "%d", &free)

	return total, free, nil
}

// VolumeExists checks if a logical volume exists
func (m *LVMManager) VolumeExists(vgName, lvName string) (bool, error) {
	lvPath := fmt.Sprintf("%s/%s", vgName, lvName)
	_, err := m.execCommand("lvs", lvPath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ResizeVolume expands a logical volume and its filesystem
func (m *LVMManager) ResizeVolume(vgName, lvName, newSize string) error {
	klog.Infof("Resizing LV %s/%s to %s", vgName, lvName, newSize)

	lvPath := fmt.Sprintf("%s/%s", vgName, lvName)

	// Resize the LV
	_, err := m.execCommand("lvextend", "-L", newSize, lvPath)
	if err != nil {
		return fmt.Errorf("failed to resize LV: %w", err)
	}

	// Resize the filesystem (works for xfs and ext4)
	devicePath := fmt.Sprintf("/dev/%s", lvPath)
	if _, err := m.execCommand("resize2fs", devicePath); err != nil {
		// Try xfs_growfs if resize2fs failed (likely XFS)
		if _, err := m.execCommand("xfs_growfs", devicePath); err != nil {
			return fmt.Errorf("failed to resize filesystem: %w", err)
		}
	}

	return nil
}
