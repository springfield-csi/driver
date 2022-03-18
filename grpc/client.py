
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

from __future__ import print_function

import logging

import grpc
import csi_pb2
import csi_pb2_grpc
import argparse


def run():
    parser = argparse.ArgumentParser()

    parser.add_argument('--command', dest='command', choices=['create', 'delete',
                                                              'publishvolume', 'unpublishvolume',
                                                              'validatevolumecapabilities', 'listvolumes', 'getcapacity',
                                                              'getcapabilities', 'createsnapshot', 'deletesnapshot',
                                                              'listsnapshots', 'expandvolume', 'getvolume'],

                        help="storage command('create', 'delete', 'publishvolume', 'unpublishvolume',\
                                              'validatevolumecapabilities', 'listvolumes', 'getcapacity',\
                                              'getcapabilities', 'createsnapshot', 'deletesnapshot',\
                                              'listsnapshots', 'expandvolume', 'getvolume'", default="list")

    parser.add_argument('--devices', dest='devices',
                        type=str, nargs='*', help='device list')
    parser.add_argument('--name', dest='name', type=str, help='volume name')
    parser.add_argument('--size',  dest='size', type=str,
                        help='volume size', default=0)
    parser.add_argument('--addr',  dest='addr', type=str,
                        help='ip address to listen', default="'localhost:")
    parser.add_argument('--port',  dest='port', type=str,
                        help='port to listen', default='localhost:50024')

    args = parser.parse_args()

    command = args.command
    dev_list = args.devices
    name = args.name
    size = args.size
    addr = args.addr

    with grpc.insecure_channel(args.port,
                               options=[('grpc.lb_policy_name', 'pick_first'),
                                        ('grpc.enable_retries', 0),
                                        ('grpc.keepalive_timeout_ms', 10000)
                                        ]) as channel:
        stub = csi_pb2_grpc.ControllerStub(channel)

        if command == "create":
            response = stub.CreateVolume(csi_pb2.CreateVolumeRequest(),
                                         timeout=30)
            print(response.message)
        elif command == "delete":
            response = stub.DeleteVolume(csi_pb2.DeleteVolumeRequest(command=command, name=name),
                                         timeout=30)
            print(response.message)
        elif command == "publishvolume":
            response = stub.ControllerPublishVolume(csi_pb2.ControllerPublishVolumeRequest(command=command),
                                                    timeout=30)
        elif command == "unpublishvolume":
            response = stub.ControllerUnpublishVolume(csi_pb2.ControllerUnpublishVolumeRequest(command=command),
                                                      timeout=30)
        elif command == "validatevolumecapabilities":
            response = stub.ValidateVolumeCapabilities(csi_pb2.ValidateVolumeCapabilitiesRequest(command=command),
                                                       timeout=30)
        elif command == "listvolumes":
            response = stub.ListVolumes(csi_pb2.ListVolumes(command=command),
                                        timeout=30)
            print(response.device_json)
            print(response.message)
        elif command == "getcapacity":
            response = stub.GetCapacity(csi_pb2.GetCapacityRequest(command=command),
                                        timeout=30)
        elif command == "getcapabilities":
            response = stub.ControllerGetCapabilities(csi_pb2.ControllerGetCapabilitiesRequest(command=command),
                                                      timeout=30)
        elif command == "createsnapshot":
            response = stub.CreateSnapshot(csi_pb2.CreateSnapshotRequest(command=command),
                                           timeout=30)
        elif command == "deletesnapshot":
            response = stub.DeleteSnapshot(csi_pb2.DeleteSnapshotRequest(command=command),
                                           timeout=30)
        elif command == "listsnapshots":
            response = stub.ListSnapshots(csi_pb2.ListSnapshotsRequest(command=command),
                                          timeout=30)
        elif command == "expandvolume":
            response = stub.ControllerExpandVolume(csi_pb2.ControllerExpandVolumeRequest(command=command),
                                                   timeout=30)
        elif command == "getvolume":
            response = stub.ControllerGetVolume(csi_pb2.ControllerGetVolumeRequest(command=command),
                                                timeout=30)
        elif command == "listdevices":
            response = stub.ListDevices(csi_pb2.StorageRequest(command=command),
                                        timeout=30)
            print(response.device_json)
            print(response.message)

    print("client received: " + response.message)


if __name__ == '__main__':
    logging.basicConfig()
    run()
