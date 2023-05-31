# Copyright (C) 2023  Red Hat, Inc.
#
# This copyrighted material is made available to anyone wishing to use,
# modify, copy, or redistribute it subject to the terms and conditions of
# the GNU General Public License v.2, or (at your option) any later version.
# This program is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY expressed or implied, including the implied warranties of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General
# Public License for more details.  You should have received a copy of the
# GNU General Public License along with this program; if not, write to the
# Free Software Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA
# 02110-1301, USA.  Any Red Hat trademarks that are incorporated in the
# source code or documentation are not subject to the GNU General Public
# License and may only be used or replicated with the express permission of
# Red Hat, Inc.
#
# Red Hat Author(s): Todd Gill <tgill@redhat.com>
#

import argparse
import sys
import sh
import dbus
import logging

logger = logging.getLogger("springfield-csi")

class StorageType():
    DEVICE_TYPE_LVM = 0
    DEVICE_TYPE_MD = 1
    DEVICE_TYPE_PARTITION = 2
    DEVICE_TYPE_BTRFS = 3
    DEVICE_TYPE_DISK = 4
    DEVICE_TYPE_LVM_THINP = 5
    DEVICE_TYPE_LVM_VDO = 6
    DEVICE_TYPE_STRATIS = 7

OBJECT_MANAGER = "org.freedesktop.DBus.ObjectManager"
BUS = dbus.SystemBus()
BUS_NAME = "com.redhat.Blivet0"
TOP_OBJECT = "/com/redhat/Blivet0/Blivet"


REVISION_NUMBER = 1
REVISION = f"r{REVISION_NUMBER}"


TIMEOUT = 10 * 1000

# TODO: Add revision to dbus interface.  Once completed, update to:
# f"{BUS_NAME}.Blivet.{REVISION}"
BLIVET_INTERFACE = f"{BUS_NAME}.Blivet"
DEVICE_INTERFACE = f"{BUS_NAME}.Device"
FORMAT_INTERFACE = f"{BUS_NAME}.Format"
PROPERTIES_INTERFACE = "org.freedesktop.DBus.Properties"

top_object = BUS.get_object(BUS_NAME, TOP_OBJECT)

blivet_interface = dbus.Interface(
    top_object,
    BLIVET_INTERFACE,
)

properties_interface = dbus.Interface(
    top_object,
    PROPERTIES_INTERFACE,
)

def get_managed_objects():
    """
    Get managed objects for /com/redhat/Blivet0
    :return: dict of object paths, objects
    """
    # TODO: GetManagedObjects is implemented at the /com/redhat/Blivet0 level.
    # The other methods are implemented at /com/redhat/Blivet0/Blivet.  Is
    # that intentional?
    logger.info("get_managed_objects()")
    object_manager = dbus.Interface(
        BUS.get_object(BUS_NAME, "/com/redhat/Blivet0"),
        OBJECT_MANAGER,
    )
    return object_manager.GetManagedObjects(timeout=TIMEOUT)


def remove_device(path, blockdevs_list, mountpoint):
    logger.info("remove_device()")

    if mountpoint:
        try:
            sh.umount(mountpoint)
        except:
            pass

    blivet_interface.RemoveDevice(path)

    blivet_interface.Commit()

    # object_paths = get_object_paths(blockdevs_list)

    # print("To Use", object_paths)

    # for disk_object_path in object_paths:
    #     blivet_interface.InitializeDisk(disk_object_path)

    # blivet_interface.Commit()


def list_device_objects():
    logger.info("list_device_objects()")
    managed_objects = get_managed_objects().items()

    return_objects = [
        obj_data[DEVICE_INTERFACE]
        for _, obj_data in managed_objects
        if DEVICE_INTERFACE in obj_data
    ]
    return return_objects


def print_dict(dict_type, print_dict):
    print(dict_type)
    for key, value in print_dict.items():
        print("    ", key, "\t: ", value)


def print_properties(path, interface):
    path = BUS.get_object(BUS_NAME, path)
    properties_interface = dbus.Interface(
        path, dbus_interface="org.freedesktop.DBus.Properties"
    )
    props = properties_interface.GetAll(interface)
    print_dict(interface, props)


def get_property(path, interface, value):
    logger.info("get_property()")
    property_object = BUS.get_object(BUS_NAME, path)
    properties_interface = dbus.Interface(
        property_object, dbus_interface="org.freedesktop.DBus.Properties"
    )
    props = properties_interface.GetAll(interface)
    
    return props.get(value)
    
def lvm_create(disk_list, fs_name, size):
    logger.info("lvm_create()")
    kwargs = {
        "device_type": StorageType.DEVICE_TYPE_LVM,
        "size": size,
        "disks": disk_list,
        "fstype": "xfs",
        "name": fs_name,
        "raid_level": "raid1",
    }

    return blivet_interface.Factory(kwargs)


def btrfs_create(disk_list, fs_name, size):
    logger.info("btrfs_create()")
    kwargs = {
        "device_type": StorageType.DEVICE_TYPE_BTRFS,
        "size": size,
        "disks": disk_list,
        "name": fs_name,
        "raid_level": "raid1",
        "container_raid_level": "raid1",
        "fstype": "btrfs",
    }

    return blivet_interface.Factory(kwargs)


def md_create(disk_list, fs_name, size):
    logger.info("md_create()")
    kwargs = {
        "device_type": StorageType.DEVICE_TYPE_MD,
        "size": size,
        "disks": disk_list,
        "fstype": "xfs",
        "name": fs_name,
        "raid_level": "raid1",
        "container_raid_level": "raid1",
    }

    return blivet_interface.Factory(kwargs)


def stratis_create(disk_list, fs_name, size):
    logger.info("stratis_create()")
    kwargs = {
        "device_type": StorageType.DEVICE_TYPE_STRATIS,
        "size": size,
        "disks": disk_list,
        "name": fs_name,
    }

    return blivet_interface.Factory(kwargs)

def get_object_paths(blockdevs_list):
    logger.info("get_object_paths()")
    objects = get_managed_objects()

    object_paths = list()
    for object_path in blivet_interface.ListDevices():
        device = objects[object_path][DEVICE_INTERFACE]
        #print_properties(object_path, DEVICE_INTERFACE)
        #print_properties(device["Format"], FORMAT_INTERFACE)

        # search for the disk object paths
        for to_use in blockdevs_list:
            if to_use == get_property(object_path, DEVICE_INTERFACE, "Path"):
                object_paths.append(str(object_path))

    print("object paths:", object_paths)
    return object_paths

def initialize_disks(blockdevs_list):
    
    object_paths = get_object_paths(blockdevs_list)

    print("To Use:", object_paths)

    for disk_object_path in object_paths:
        blivet_interface.InitializeDisk(disk_object_path)
        print_properties(disk_object_path, DEVICE_INTERFACE)

    blivet_interface.Commit()
    print("initialize_disks: blivet_interface.Commit() completed")
    return object_paths

def reset():
    blivet_interface.Reset()

def fs_destroy(path, disks):
    logger.info("fs_destroy()")
    mount_point = get_property(newdev_object_path, DEVICE_INTERFACE, "Mountpoint")
    remove_device(path, disks, mount_point)

def fs_create(name, disks_list, storage_type, size):
    logger.info("fs_create()")
    
    disk_object_paths = initialize_disks(disks_list)
    newdev_object_path = None

    if storage_type == StorageType.DEVICE_TYPE_LVM:
        newdev_object_path = lvm_create(disk_object_paths, name, size)
    elif storage_type == StorageType.DEVICE_TYPE_BTRFS:
        newdev_object_path = btrfs_create(disk_object_paths, name, size)
    elif storage_type == StorageType.DEVICE_TYPE_MD:
        newdev_object_path = md_create(disk_object_paths, name, size)
    elif storage_type == StorageType.DEVICE_TYPE_STRATIS:
        newdev_object_path = stratis_create(disk_object_paths, name, size)

    blivet_interface.Commit()
    return newdev_object_path


if __name__ == "__main__":

    props = properties_interface.GetAll(BLIVET_INTERFACE)
    print_dict(BLIVET_INTERFACE, props)

    blivet_interface.Reset()
    blockdevs_list = list(["", "", ""])
    new_object_path = fs_create("test_fs", blockdevs_list, StorageType.DEVICE_TYPE_LVM, "3GB")
    mount_point = get_property(new_object_path, DEVICE_INTERFACE, "Mountpoint")
    remove_device(new_object_path, blockdevs_list, mount_point)
    
    blivet_interface.Reset()

    new_object_path = fs_create("test_fs", blockdevs_list, StorageType.DEVICE_TYPE_LVM, "3GB")
    mount_point = get_property(new_object_path, DEVICE_INTERFACE, "Mountpoint")
    remove_device(new_object_path, blockdevs_list, mount_point)
