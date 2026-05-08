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

package blivet

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"k8s.io/klog/v2"
)

// StorageType represents different storage device types supported by Blivet
type StorageType int

const (
	DeviceTypeLVM       StorageType = 0
	DeviceTypeMD        StorageType = 1
	DeviceTypePartition StorageType = 2
	DeviceTypeBTRFS     StorageType = 3
	DeviceTypeDisk      StorageType = 4
	DeviceTypeLVMThinP  StorageType = 5
	DeviceTypeLVMVDO    StorageType = 6
	DeviceTypeStratis   StorageType = 7
)

const (
	busName           = "com.redhat.Blivet0"
	topObject         = "/com/redhat/Blivet0/Blivet"
	blivetInterface   = "com.redhat.Blivet0.Blivet"
	deviceInterface   = "com.redhat.Blivet0.Device"
	formatInterface   = "com.redhat.Blivet0.Format"
	objectManager     = "org.freedesktop.DBus.ObjectManager"
	propertiesIface   = "org.freedesktop.DBus.Properties"
	defaultTimeout    = 600 // seconds
)

// Client is a DBus client for interacting with the Blivet storage service
type Client struct {
	conn            *dbus.Conn
	blivetObj       dbus.BusObject
	blivetInterface dbus.BusObject
}

// NewClient creates a new Blivet DBus client
func NewClient() (*Client, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to system bus: %w", err)
	}

	obj := conn.Object(busName, topObject)

	return &Client{
		conn:            conn,
		blivetObj:       obj,
		blivetInterface: obj,
	}, nil
}

// Close closes the DBus connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Reset resets the Blivet state and rescans the system
func (c *Client) Reset() error {
	klog.V(4).Info("Calling Blivet Reset")
	call := c.blivetInterface.Call(blivetInterface+".Reset", 0)
	if call.Err != nil {
		return fmt.Errorf("reset failed: %w", call.Err)
	}
	return nil
}

// GetManagedObjects returns all managed objects from Blivet
func (c *Client) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	klog.V(4).Info("Getting managed objects")

	obj := c.conn.Object(busName, "/com/redhat/Blivet0")
	call := obj.Call(objectManager+".GetManagedObjects", 0)
	if call.Err != nil {
		return nil, fmt.Errorf("failed to get managed objects: %w", call.Err)
	}

	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := call.Store(&objects); err != nil {
		return nil, fmt.Errorf("failed to parse managed objects: %w", err)
	}

	return objects, nil
}

// GetProperty retrieves a property from a DBus object
func (c *Client) GetProperty(objectPath dbus.ObjectPath, iface, property string) (interface{}, error) {
	obj := c.conn.Object(busName, objectPath)
	variant, err := obj.GetProperty(iface + "." + property)
	if err != nil {
		return nil, fmt.Errorf("failed to get property %s.%s: %w", iface, property, err)
	}
	return variant.Value(), nil
}

// ResolveDevice resolves a device path to a DBus object path
func (c *Client) ResolveDevice(devicePath string) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Resolving device: %s", devicePath)

	call := c.blivetInterface.Call(blivetInterface+".ResolveDevice", 0, devicePath)
	if call.Err != nil {
		return "", fmt.Errorf("failed to resolve device %s: %w", devicePath, call.Err)
	}

	var objPath dbus.ObjectPath
	if err := call.Store(&objPath); err != nil {
		return "", fmt.Errorf("failed to parse resolved device path: %w", err)
	}

	return objPath, nil
}

// InitializeDisk initializes a disk for use
func (c *Client) InitializeDisk(objectPath dbus.ObjectPath) error {
	klog.V(4).Infof("Initializing disk: %s", objectPath)

	call := c.blivetInterface.Call(blivetInterface+".InitializeDisk", 0, objectPath)
	if call.Err != nil {
		return fmt.Errorf("failed to initialize disk %s: %w", objectPath, call.Err)
	}

	return nil
}

// Commit commits all pending changes
func (c *Client) Commit() error {
	klog.V(4).Info("Committing Blivet changes")

	call := c.blivetInterface.Call(blivetInterface+".Commit", 0)
	if call.Err != nil {
		return fmt.Errorf("commit failed: %w", call.Err)
	}

	return nil
}

// Factory creates a new storage device using Blivet's factory method
func (c *Client) Factory(params map[string]interface{}) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Creating device with factory: %+v", params)

	call := c.blivetInterface.Call(blivetInterface+".Factory", 0, params)
	if call.Err != nil {
		return "", fmt.Errorf("factory call failed: %w", call.Err)
	}

	var objPath dbus.ObjectPath
	if err := call.Store(&objPath); err != nil {
		return "", fmt.Errorf("failed to parse factory result: %w", err)
	}

	return objPath, nil
}

// RemoveDevice removes a device
func (c *Client) RemoveDevice(objectPath dbus.ObjectPath) error {
	klog.V(4).Infof("Removing device: %s", objectPath)

	call := c.blivetInterface.Call(blivetInterface+".RemoveDevice", 0, objectPath)
	if call.Err != nil {
		return fmt.Errorf("failed to remove device %s: %w", objectPath, call.Err)
	}

	return nil
}

// FormatTeardown tears down a filesystem format
func (c *Client) FormatTeardown(formatPath dbus.ObjectPath) error {
	klog.V(4).Infof("Tearing down format: %s", formatPath)

	obj := c.conn.Object(busName, formatPath)
	call := obj.Call(formatInterface+".Teardown", 0)
	if call.Err != nil {
		return fmt.Errorf("failed to teardown format %s: %w", formatPath, call.Err)
	}

	return nil
}

// InitializeDisks initializes multiple disks for use
func (c *Client) InitializeDisks(devicePaths []string) ([]dbus.ObjectPath, error) {
	var objectPaths []dbus.ObjectPath

	for _, devicePath := range devicePaths {
		objPath, err := c.ResolveDevice(devicePath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve device %s: %w", devicePath, err)
		}
		objectPaths = append(objectPaths, objPath)
	}

	for _, objPath := range objectPaths {
		if err := c.InitializeDisk(objPath); err != nil {
			return nil, fmt.Errorf("failed to initialize disk %s: %w", objPath, err)
		}
	}

	if err := c.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit disk initialization: %w", err)
	}

	klog.V(4).Infof("Initialized disks: %v", objectPaths)
	return objectPaths, nil
}

// CreateLVM creates an LVM volume
func (c *Client) CreateLVM(name string, diskPaths []dbus.ObjectPath, size string) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Creating LVM volume: %s", name)

	params := map[string]interface{}{
		"device_type": int32(DeviceTypeLVM),
		"size":        size,
		"disks":       diskPaths,
		"fstype":      "xfs",
		"name":        name,
		"raid_level":  "raid1",
	}

	return c.Factory(params)
}

// CreateBTRFS creates a BTRFS volume
func (c *Client) CreateBTRFS(name string, diskPaths []dbus.ObjectPath, size string) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Creating BTRFS volume: %s", name)

	params := map[string]interface{}{
		"device_type":          int32(DeviceTypeBTRFS),
		"size":                 size,
		"disks":                diskPaths,
		"name":                 name,
		"fstype":               "btrfs",
		"raid_level":           "raid0",
		"container_raid_level": "raid0",
	}

	return c.Factory(params)
}

// CreateMD creates an MD RAID volume
func (c *Client) CreateMD(name string, diskPaths []dbus.ObjectPath, size string) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Creating MD volume: %s", name)

	params := map[string]interface{}{
		"device_type":          int32(DeviceTypeMD),
		"size":                 size,
		"disks":                diskPaths,
		"fstype":               "xfs",
		"name":                 name,
		"raid_level":           "raid1",
		"container_raid_level": "raid1",
	}

	return c.Factory(params)
}

// CreateStratis creates a Stratis volume
func (c *Client) CreateStratis(name string, diskPaths []dbus.ObjectPath, size string) (dbus.ObjectPath, error) {
	klog.V(4).Infof("Creating Stratis volume: %s", name)

	params := map[string]interface{}{
		"device_type": int32(DeviceTypeStratis),
		"size":        size,
		"disks":       diskPaths,
		"name":        name,
	}

	return c.Factory(params)
}

// CreateVolume creates a volume based on the storage type
func (c *Client) CreateVolume(name string, devicePaths []string, storageType StorageType, size string) (dbus.ObjectPath, string, error) {
	diskPaths, err := c.InitializeDisks(devicePaths)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize disks: %w", err)
	}

	var objPath dbus.ObjectPath
	switch storageType {
	case DeviceTypeLVM:
		objPath, err = c.CreateLVM(name, diskPaths, size)
	case DeviceTypeBTRFS:
		objPath, err = c.CreateBTRFS(name, diskPaths, size)
	case DeviceTypeMD:
		objPath, err = c.CreateMD(name, diskPaths, size)
	case DeviceTypeStratis:
		objPath, err = c.CreateStratis(name, diskPaths, size)
	default:
		return "", "", fmt.Errorf("unsupported storage type: %d", storageType)
	}

	if err != nil {
		return "", "", fmt.Errorf("failed to create volume: %w", err)
	}

	if err := c.Commit(); err != nil {
		return "", "", fmt.Errorf("failed to commit volume creation: %w", err)
	}

	// Get the block device path
	blockPath, err := c.GetProperty(objPath, deviceInterface, "Path")
	if err != nil {
		return "", "", fmt.Errorf("failed to get block device path: %w", err)
	}

	blockPathStr, ok := blockPath.(string)
	if !ok {
		return "", "", fmt.Errorf("block path is not a string: %v", blockPath)
	}

	klog.V(4).Infof("Created volume %s at %s", name, blockPathStr)
	return objPath, blockPathStr, nil
}

// DeleteVolume removes a volume and its underlying devices
func (c *Client) DeleteVolume(objectPath dbus.ObjectPath, devicePaths []string) error {
	klog.V(4).Infof("Deleting volume: %s", objectPath)

	// Get format and tear it down
	formatPath, err := c.GetProperty(objectPath, deviceInterface, "Format")
	if err != nil {
		klog.Warningf("Failed to get format for device %s: %v", objectPath, err)
	} else if formatPath != nil {
		if formatObjPath, ok := formatPath.(dbus.ObjectPath); ok {
			if err := c.FormatTeardown(formatObjPath); err != nil {
				klog.Warningf("Failed to teardown format: %v", err)
			}
		}
	}

	// Remove the main device
	if err := c.RemoveDevice(objectPath); err != nil {
		return fmt.Errorf("failed to remove device: %w", err)
	}

	// Remove underlying block devices
	for _, devPath := range devicePaths {
		objPath, err := c.ResolveDevice(devPath)
		if err != nil {
			klog.Warningf("Failed to resolve device %s for removal: %v", devPath, err)
			continue
		}

		if err := c.RemoveDevice(objPath); err != nil {
			klog.Warningf("Failed to remove device %s: %v", objPath, err)
		}
	}

	if err := c.Commit(); err != nil {
		return fmt.Errorf("failed to commit volume deletion: %w", err)
	}

	return nil
}
