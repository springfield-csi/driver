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
	"strings"

	"k8s.io/klog/v2"
)

// MDManager handles MD RAID operations
type MDManager struct {
	executor *Executor
}

// NewMDManager creates a new MD RAID manager
func NewMDManager(executor *Executor) *MDManager {
	return &MDManager{
		executor: executor,
	}
}

// CreateArray creates an MD RAID array
func (m *MDManager) CreateArray(name string, devices []string, raidLevel string) (string, error) {
	klog.Infof("Creating MD array %s with RAID level %s on devices %v", name, raidLevel, devices)

	if len(devices) < 2 {
		return "", fmt.Errorf("MD RAID requires at least 2 devices, got %d", len(devices))
	}

	// Wipe devices first
	for _, dev := range devices {
		klog.V(4).Infof("Wiping device %s", dev)
		if _, err := m.executor.Run("wipefs", "-af", dev); err != nil {
			klog.Warningf("Failed to wipe device %s: %v", dev, err)
		}
	}

	// Determine device name
	mdDevice := fmt.Sprintf("/dev/md/%s", name)

	// Create the array
	args := []string{
		"--create", mdDevice,
		"--level=" + raidLevel,
		fmt.Sprintf("--raid-devices=%d", len(devices)),
		"--metadata=1.2",
		"--name=" + name,
	}
	args = append(args, devices...)

	if _, err := m.executor.Run("mdadm", args...); err != nil {
		return "", fmt.Errorf("failed to create MD array: %w", err)
	}

	// Wait for array to be ready
	if _, err := m.executor.Run("mdadm", "--wait", mdDevice); err != nil {
		klog.Warningf("mdadm --wait failed: %v", err)
	}

	klog.Infof("Created MD array at %s", mdDevice)
	return mdDevice, nil
}

// CreateFilesystem creates a filesystem on the MD device
func (m *MDManager) CreateFilesystem(device, fsType string) error {
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

	if _, err := m.executor.Run(cmd, args...); err != nil {
		return fmt.Errorf("failed to create %s filesystem: %w", fsType, err)
	}

	return nil
}

// DeleteArray deletes an MD RAID array
func (m *MDManager) DeleteArray(name string) error {
	klog.Infof("Deleting MD array %s", name)

	mdDevice := fmt.Sprintf("/dev/md/%s", name)

	// Stop the array
	if _, err := m.executor.Run("mdadm", "--stop", mdDevice); err != nil {
		// Try with /dev/mdX format
		stdout, _ := m.executor.Run("cat", "/proc/mdstat")
		if strings.Contains(stdout, name) {
			// Find the md device number
			lines := strings.Split(stdout, "\n")
			for _, line := range lines {
				if strings.Contains(line, name) {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						mdNum := fields[0]
						altDevice := "/dev/" + mdNum
						if _, err := m.executor.Run("mdadm", "--stop", altDevice); err != nil {
							return fmt.Errorf("failed to stop MD array: %w", err)
						}
						break
					}
				}
			}
		} else {
			return fmt.Errorf("failed to stop MD array: %w", err)
		}
	}

	klog.Infof("Deleted MD array %s", name)
	return nil
}

// ArrayExists checks if an MD array exists
func (m *MDManager) ArrayExists(name string) (bool, error) {
	stdout, err := m.executor.Run("cat", "/proc/mdstat")
	if err != nil {
		return false, err
	}

	return strings.Contains(stdout, name), nil
}
