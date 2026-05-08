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

// StratisManager handles Stratis operations
type StratisManager struct {
	executor *Executor
}

// NewStratisManager creates a new Stratis manager
func NewStratisManager(executor *Executor) *StratisManager {
	return &StratisManager{
		executor: executor,
	}
}

// CreatePool creates a Stratis pool
func (m *StratisManager) CreatePool(poolName string, devices []string) error {
	klog.Infof("Creating Stratis pool %s on devices %v", poolName, devices)

	if len(devices) < 1 {
		return fmt.Errorf("Stratis requires at least 1 device")
	}

	// Check if pool already exists
	exists, err := m.PoolExists(poolName)
	if err != nil {
		return fmt.Errorf("failed to check pool existence: %w", err)
	}
	if exists {
		klog.V(4).Infof("Pool %s already exists", poolName)
		return nil
	}

	// Create pool
	args := append([]string{"pool", "create", poolName}, devices...)
	if _, err := m.executor.Run("stratis", args...); err != nil {
		return fmt.Errorf("failed to create Stratis pool: %w", err)
	}

	return nil
}

// CreateFilesystem creates a Stratis filesystem
func (m *StratisManager) CreateFilesystem(poolName, fsName string) (string, error) {
	klog.Infof("Creating Stratis filesystem %s in pool %s", fsName, poolName)

	// Create filesystem
	if _, err := m.executor.Run("stratis", "filesystem", "create", poolName, fsName); err != nil {
		return "", fmt.Errorf("failed to create Stratis filesystem: %w", err)
	}

	// Return device path
	devicePath := fmt.Sprintf("/dev/stratis/%s/%s", poolName, fsName)
	return devicePath, nil
}

// DeleteFilesystem deletes a Stratis filesystem
func (m *StratisManager) DeleteFilesystem(poolName, fsName string) error {
	klog.Infof("Deleting Stratis filesystem %s from pool %s", fsName, poolName)

	if _, err := m.executor.Run("stratis", "filesystem", "destroy", poolName, fsName); err != nil {
		return fmt.Errorf("failed to delete Stratis filesystem: %w", err)
	}

	return nil
}

// DeletePool deletes a Stratis pool
func (m *StratisManager) DeletePool(poolName string) error {
	klog.Infof("Deleting Stratis pool %s", poolName)

	if _, err := m.executor.Run("stratis", "pool", "destroy", poolName); err != nil {
		return fmt.Errorf("failed to delete Stratis pool: %w", err)
	}

	return nil
}

// PoolExists checks if a Stratis pool exists
func (m *StratisManager) PoolExists(poolName string) (bool, error) {
	stdout, err := m.executor.Run("stratis", "pool", "list")
	if err != nil {
		return false, err
	}

	return strings.Contains(stdout, poolName), nil
}

// FilesystemExists checks if a Stratis filesystem exists
func (m *StratisManager) FilesystemExists(poolName, fsName string) (bool, error) {
	stdout, err := m.executor.Run("stratis", "filesystem", "list", poolName)
	if err != nil {
		// Pool might not exist
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	return strings.Contains(stdout, fsName), nil
}

// IsAvailable checks if Stratis is available on the system
func (m *StratisManager) IsAvailable() bool {
	return m.executor.CommandExists("stratis")
}
