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

package storage

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

// BTRFSManager handles BTRFS operations
type BTRFSManager struct {
	executor *Executor
	baseDir  string // Base directory for BTRFS filesystems
}

// NewBTRFSManager creates a new BTRFS manager
func NewBTRFSManager(executor *Executor) *BTRFSManager {
	return &BTRFSManager{
		executor: executor,
		baseDir:  "/var/lib/springfield-csi/btrfs",
	}
}

// CreateFilesystem creates a BTRFS filesystem
func (m *BTRFSManager) CreateFilesystem(name string, devices []string, dataProfile, metadataProfile string) (string, error) {
	klog.Infof("Creating BTRFS filesystem %s on devices %v", name, devices)

	if len(devices) < 1 {
		return "", fmt.Errorf("BTRFS requires at least 1 device")
	}

	// Wipe devices
	for _, dev := range devices {
		if _, err := m.executor.Run("wipefs", "-af", dev); err != nil {
			klog.Warningf("Failed to wipe device %s: %v", dev, err)
		}
	}

	// Build mkfs.btrfs command
	args := []string{"-f"}

	if dataProfile != "" {
		args = append(args, "-d", dataProfile)
	}
	if metadataProfile != "" {
		args = append(args, "-m", metadataProfile)
	}

	// Add label
	args = append(args, "-L", name)

	// Add devices
	args = append(args, devices...)

	if _, err := m.executor.Run("mkfs.btrfs", args...); err != nil {
		return "", fmt.Errorf("failed to create BTRFS filesystem: %w", err)
	}

	// Return the first device as the mount source
	return devices[0], nil
}

// CreateSubvolume creates a BTRFS subvolume
func (m *BTRFSManager) CreateSubvolume(mountpoint, name string) (string, error) {
	klog.Infof("Creating BTRFS subvolume %s at %s", name, mountpoint)

	subvolPath := filepath.Join(mountpoint, name)

	if _, err := m.executor.Run("btrfs", "subvolume", "create", subvolPath); err != nil {
		return "", fmt.Errorf("failed to create subvolume: %w", err)
	}

	return subvolPath, nil
}

// DeleteSubvolume deletes a BTRFS subvolume
func (m *BTRFSManager) DeleteSubvolume(subvolPath string) error {
	klog.Infof("Deleting BTRFS subvolume %s", subvolPath)

	if _, err := m.executor.Run("btrfs", "subvolume", "delete", subvolPath); err != nil {
		return fmt.Errorf("failed to delete subvolume: %w", err)
	}

	return nil
}

// DeleteFilesystem removes a BTRFS filesystem by wiping the devices
func (m *BTRFSManager) DeleteFilesystem(device string) error {
	klog.Infof("Deleting BTRFS filesystem on %s", device)

	// Wipe the device
	if _, err := m.executor.Run("wipefs", "-af", device); err != nil {
		return fmt.Errorf("failed to wipe BTRFS device: %w", err)
	}

	return nil
}

// FilesystemExists checks if a BTRFS filesystem exists on a device
func (m *BTRFSManager) FilesystemExists(device string) (bool, error) {
	stdout, stderr, err := m.executor.RunWithStderr("blkid", "-o", "value", "-s", "TYPE", device)
	if err != nil {
		// Device might not exist or not have a filesystem
		if strings.Contains(stderr, "No such file") || strings.Contains(stderr, "Inappropriate ioctl") {
			return false, nil
		}
		return false, err
	}

	return strings.TrimSpace(stdout) == "btrfs", nil
}

// GetFilesystemInfo returns information about a BTRFS filesystem
func (m *BTRFSManager) GetFilesystemInfo(mountpoint string) (map[string]string, error) {
	stdout, err := m.executor.Run("btrfs", "filesystem", "show", mountpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get filesystem info: %w", err)
	}

	info := make(map[string]string)
	info["raw_output"] = stdout

	return info, nil
}
