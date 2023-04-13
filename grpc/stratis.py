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
#

import os
import dbus
import logging

STRATIS_PATH="/stratis/stratis/"
OBJECT_MANAGER = "org.freedesktop.DBus.ObjectManager"
BUS = dbus.SystemBus()
BUS_NAME = "org.storage.stratis3"
TOP_OBJECT = "/org/storage/stratis3"
REVISION_NUMBER = 2
REVISION = f"r{REVISION_NUMBER}"
TIMEOUT = 10 * 1000

MNGR_IFACE = f"{BUS_NAME}.Manager.{REVISION}"
REPORT_IFACE = f"{BUS_NAME}.Report.{REVISION}"
POOL_IFACE = f"{BUS_NAME}.pool.{REVISION}"
FS_IFACE = f"{BUS_NAME}.filesystem.{REVISION}"
BLKDEV_IFACE = f"{BUS_NAME}.blockdev.{REVISION}"

CONTAINER_POOL = os.getenv("CONTAINER_POOL", "springfield-csi-pool")

logger = logging.getLogger("LOGGER_NAME")

def get_managed_objects():
    """
    Get managed objects for stratis
    :return: A dict,  Keys are object paths with dicts containing interface
                        names mapped to property dicts.
                        Property dicts map names to values.
    """
    object_manager = dbus.Interface(
        BUS.get_object(BUS_NAME, TOP_OBJECT),
        OBJECT_MANAGER,
    )
    return object_manager.GetManagedObjects(timeout=TIMEOUT)


def pool_object_path(pool_name):
    """
    Query the pools
    :return: the dbus object path for the pool
    :rtype: str
    """

    for obj_path, obj_data in get_managed_objects().items():
        if POOL_IFACE in obj_data and str(obj_data[POOL_IFACE]["Name"]) == pool_name:
            return obj_path

    return None


def fs_create(pool_path, fs_name, *, fs_size=None):
    """
    Create a filesystem
    :param str pool_path: The object path of the pool in which the filesystem will be created
    :param str fs_name: The name of the filesystem to create
    :param str fs_size: The size of the filesystem to create
    :return: The return values of the CreateFilesystems call
    :rtype: The D-Bus types (ba(os)), q, and s
    """
    iface = dbus.Interface(
        BUS.get_object(BUS_NAME, pool_path),
        POOL_IFACE,
    )

    file_spec = (
        (fs_name, (False, "")) if fs_size is None else (fs_name, (True, fs_size))
    )

    return iface.CreateFilesystems([file_spec], timeout=TIMEOUT)


def fs_destroy(pool_name, fs_name):
    """
    Destroy a filesystem
    :param str pool_name: The name of the pool which contains the filesystem
    :param str fs_name: The name of the filesystem to destroy
    :return: The return values of the DestroyFilesystems call, or None
    :rtype: The D-Bus types (bas), q, and s, or None
    """
    objects = get_managed_objects().items()

    pool_objects = {
        path: obj_data[POOL_IFACE]
        for path, obj_data in objects
        if POOL_IFACE in obj_data
    }
    fs_objects = {
        path: obj_data[FS_IFACE]
        for path, obj_data in objects
        if FS_IFACE in obj_data
    }

    pool_paths = [
        path
        for path, pool_obj in pool_objects.items()
        if pool_obj["Name"] == pool_name
    ]
    if len(pool_paths) != 1:
        return None

    pool_path = pool_paths[0]

    fs_paths = [
        path
        for path, fs_obj in fs_objects.items()
        if fs_obj["Name"] == fs_name and fs_obj["Pool"] == pool_path
    ]
    if len(fs_paths) != 1:
        return None

    iface = dbus.Interface(
        BUS.get_object(BUS_NAME, pool_path),
        POOL_IFACE,
    )
    return iface.DestroyFilesystems(fs_paths, timeout=TIMEOUT)


def pool_create(
    pool_name, devices, *, key_desc=None, clevis_info=None, redundancy=None
):
    """
    Create a pool
    :param str pool_name: The name of the pool to create
    :param str devices: A list of devices that can be used to create the pool
    :param key_desc: Key description
    :type key_desc: str or NoneType
    :param clevis_info: pin identifier and JSON clevis configuration
    :type clevis_info: str * str OR NoneType
    :return: The return values of the CreatePool call
    :rtype: The D-Bus types (b(oao)), q, and s
    """
    iface = dbus.Interface(
        BUS.get_object(BUS_NAME, TOP_OBJECT),
        MNGR_IFACE,
    )
    return iface.CreatePool(
        pool_name,
        (True, redundancy) if redundancy is not None else (False, 0),
        devices,
        (True, key_desc) if key_desc is not None else (False, ""),
        (True, clevis_info) if clevis_info is not None else (False, ("", "")),
        timeout=TIMEOUT,
    )


def fs_create(pool_path, fs_name, *, fs_size=None):
    """
    Create a filesystem
    :param str pool_path: The object path of the pool in which the filesystem will be created
    :param str fs_name: The name of the filesystem to create
    :param str fs_size: The size of the filesystem to create
    :return: The return values of the CreateFilesystems call
    :rtype: The D-Bus types (ba(os)), q, and s
    """
    iface = dbus.Interface(
        BUS.get_object(BUS_NAME, pool_path),
        POOL_IFACE,
    )

    file_spec = (
        (fs_name, (False, "")) if fs_size is None else (fs_name, (True, fs_size))
    )

    (
        (
            filesystems_created,
            (array_of_tuples_with_obj_paths_and_names),
        ),
        return_code,
        msg,
    ) =  iface.CreateFilesystems([file_spec], timeout=TIMEOUT)

    logger.info(array_of_tuples_with_obj_paths_and_names)

    return array_of_tuples_with_obj_paths_and_names[0][0], array_of_tuples_with_obj_paths_and_names[0][1]


def fs_list():
    """
    Query the file systems
    :return: A dict,  Key being the fs name, the value being the pool name
    :rtype: dict mapping str to str
    """
    objects = get_managed_objects().items()

    fs_objects = [obj_data[FS_IFACE]
                  for _, obj_data in objects if FS_IFACE in obj_data]

    pool_path_to_name = {
        obj: obj_data[POOL_IFACE]["Name"]
        for obj, obj_data in objects
        if POOL_IFACE in obj_data
    }

    return {
        fs_object["Name"]: pool_path_to_name[fs_object["Pool"]]
        for fs_object in fs_objects
    }
