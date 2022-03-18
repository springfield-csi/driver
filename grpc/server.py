
# Copyright (C) 2022  Red Hat, Inc.
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
import concurrent.futures as futures
import logging
import argparse
import json
import grpc
import socket
from pathlib import Path
import csi_pb2_grpc

import blivet
from blivet.devices import StorageDevice
from blivet.devices import LVMLogicalVolumeDevice
from blivet.devices import LVMVolumeGroupDevice
from blivet.size import Size

from blivet.util import set_up_logging
import driver
from identity import SpringfieldIdentityService
from controller import SpringfieldControllerService
from controller import disks_to_use
from controller import blivet_handle
from controller import VOLUME_GROUP_NAME

from node import SpringfieldNodeService

STORAGE_DEVS_FILE = "storage_devs.json"

set_up_logging()


def run_server(port, addr, nodeid):

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    csi_pb2_grpc.add_ControllerServicer_to_server(
        SpringfieldControllerService(nodeid=nodeid), server)
    csi_pb2_grpc.add_IdentityServicer_to_server(
        SpringfieldIdentityService(), server)
    csi_pb2_grpc.add_NodeServicer_to_server(
        SpringfieldNodeService(nodeid=nodeid), server)

    server.add_insecure_port(addr + str(port))
    server.start()
    server.wait_for_termination()


# Note: the disks need to be initialized and the volume group created before the
# grpc server is started.

def initilize_disks(init_disks):

    path = Path(STORAGE_DEVS_FILE)

    if not path.is_file():
        logging.warning('%s file not found in %s',
                        STORAGE_DEVS_FILE, Path.cwd())
        # look in the grpc subdirectory
        path = Path("grpc/" + STORAGE_DEVS_FILE)
        if not path.is_file():
            logging.error('%s file not found in %s',
                          STORAGE_DEVS_FILE, Path.cwd())
            exit()

    try:
        with open(path) as json_file:
            storage_devs = json.load(json_file)['use_for_csi_storage']
    except ValueError:
        logging.error('Failed to parse {}', STORAGE_DEVS_FILE)
        exit()

    blivet_handle.reset()

    for dev_path in storage_devs:
        device = blivet_handle.devicetree.get_device_by_path(dev_path)

        if device == None:
            logging.warning('device %s not found', dev_path)
            continue

        if device.is_disk == True and device.is_empty == True:
            disks_to_use.append(device)
            blivet_handle.initialize_disk(device)
        else:
            logging.warning(
                'device %s not empty or not a valid device', dev_path)
            continue
    try:
        blivet_handle.do_it()

    except BaseException as error:
        logging.error('An exception occurred: {}'.format(error))
        exit()

    if len(disks_to_use) == 0:
        logging.error("No useable disks")
        exit()

    pv = StorageDevice("pv1", fmt=blivet.formats.get_format("lvmpv"),
                       size=Size("1 GiB"), exists=True)
    vg = LVMVolumeGroupDevice("testvg", parents=[pv], exists=True)
    lv1 = LVMLogicalVolumeDevice(
        "data_lv", parents=[vg], size=Size("500 MiB"), exists=False)

    lv2 = LVMLogicalVolumeDevice("metadata_lv", parents=[
        vg], size=Size("50 MiB"), exists=False)

    for dev in (pv, vg, lv1, lv2):
        b.devicetree._add_device(dev)

    # check that all the above devices are in the expected places
    self.assertEqual(set(b.devices), {pv, vg, lv1, lv2})
    self.assertEqual(set(b.vgs), {vg})
    self.assertEqual(set(b.lvs), {lv1, lv2})
    self.assertEqual(set(b.vgs[0].lvs), {lv1, lv2})

    self.assertEqual(vg.size, Size("1020 MiB"))
    self.assertEqual(lv1.size, Size("500 MiB"))
    self.assertEqual(lv2.size, Size("50 MiB"))

    # combine the two LVs into a thin pool (the LVs should become its internal LVs)
    pool = b.new_lv_from_lvs(
        vg, name="pool", seg_type="thin-pool", from_lvs=(lv1, lv2))

    device = blivet_handle.factory_device(blivet.devicefactory.DEVICE_TYPE_LVM,
                                          container_name=VOLUME_GROUP_NAME,
                                          disks=disks_to_use,
                                          container_size=blivet.devicefactory.SIZE_POLICY_AUTO)
    try:
        blivet_handle.do_it()
    except BaseException as error:
        logging.error('An exception occurred: {}'.format(error))
        exit()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--nodeid',  dest='nodeid', type=str,
                        help='unique node identifier', default=socket.getfqdn())
    parser.add_argument('--addr',  dest='addr', type=str,
                        help='ip address to listen', default="[::]:")
    parser.add_argument('--port',  dest='port', type=int,
                        help='port to listen', default=50024)
    parser.add_argument('--init_disks',  dest='init_disks', type=bool,
                        help='initialize storage', default=False)

    args = parser.parse_args()

    port = args.port
    addr = args.addr
    nodeid = args.nodeid
    init_disks = args.init_disks

    logging.basicConfig()
    initilize_disks(init_disks)
    run_server(port, addr, nodeid)
