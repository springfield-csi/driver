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

from google.protobuf.json_format import MessageToJson, MessageToDict
from csi_pb2_grpc import NodeServicer
import csi_pb2
from csi_pb2 import NodeGetCapabilitiesResponse, NodeGetInfoResponse, NodePublishVolumeResponse, NodeGetVolumeStatsResponse, NodeExpandVolumeResponse, NodeServiceCapability, NodeUnpublishVolumeResponse, Topology, VolumeUsage, VolumeCondition

from blivet.util import mount, umount

import grpc

import controller

import os
import array

# CSI Spec https://github.com/container-storage-interface/spec/blob/master/spec.md


class SpringfieldNodeService(NodeServicer):
    def __init__(self, nodeid):
        self.nodeid = nodeid

    def NodePublishVolume(self, request, context):
        if request.volume_id == None or request.volume_id == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id"
            )

        if request.target_path == None or request.target_path == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include target_path"
            )

        # if not request.volume_capability == None:
        #     context.abort(
        #         grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_capability"
        #     )
        # make sure volume exits.
        volume_map = controller.get_volume(request.volume_id)

        if volume_map == None:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Volume not found",
            )

        volume_capability = request.volume_capability
        print(volume_capability)
        fstype = volume_capability.mount.fs_type

        access_type = volume_capability.WhichOneof("access_type")
        assert access_type == "mount" or access_type == "block"

        if fstype not in ['xfs', 'btrfs', 'ext4', '']:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "Unsupported filesystem type: {fstype}",
            )

        if fstype == '':
            fstype = "xfs"

        if volume_capability.access_mode.mode not in [
                csi_pb2.VolumeCapability.AccessMode.Mode.SINGLE_NODE_WRITER]:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "Unsupported access mode: {csi_pb2.VolumeCapability.AccessMode.Mode.Name(volume_capability.access_mode.mode)}",
            )
        if access_type == "mount":
            mount(volume_map.dev_path, request.target_path, fstype)

        volume_map.published_path = request.target_path

        return NodePublishVolumeResponse()

    def NodeUnpublishVolume(self, request, context):
        if request.volume_id == None or request.volume_id == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id"
            )

        if request.target_path == None or request.target_path == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include target_path"
            )
        if not os.path.exists(request.target_path):
            context.abort(
                grpc.StatusCode.NOT_FOUND, "request.target_path does not exits"
            )
        volume_map = controller.get_volume(request.volume_id)

        try:
            umount(request.target_path)
        except OSError as e:
            self.logger.warning("Warining umount filed: %s : %s" %
                                (request.target_path, e.strerror))
        try:
            if os.path.isfile(request.target_path):
                os.remove(request.target_path)
            if os.path.isdir(request.target_path):
                os.rmdir(request.target_path)

        except OSError as e:
            self.logger.warning("Warining remove unpublish remove: %s : %s" %
                                (request.target_path, e.strerror))
        return NodeUnpublishVolumeResponse()

    def NodeGetCapabilities(self, request, context):
        get_volume_stats = NodeServiceCapability(rpc=NodeServiceCapability.RPC(
            type=NodeServiceCapability.RPC.GET_VOLUME_STATS))
        # stage_unstage = NodeServiceCapability(rpc=NodeServiceCapability.RPC(
        #     type=NodeServiceCapability.RPC.STAGE_UNSTAGE_VOLUME))
        # expand_volume = NodeServiceCapability(rpc=NodeServiceCapability.RPC(
        #     type=NodeServiceCapability.RPC.EXPAND_VOLUME))

        capabilities = [get_volume_stats]
        return NodeGetCapabilitiesResponse(capabilities=capabilities)

    def NodeGetInfo(self, request, context):
        return NodeGetInfoResponse(
            node_id=self.nodeid,
            accessible_topology=Topology(
                segments={"hostname": self.nodeid}
            ),
        )

    def NodeExpandVolume(self, request, context):
        self.logger.warning(
            "NodeExpandVolume called, which is not implemented."
        )

        return NodeExpandVolumeResponse()

    def NodeGetVolumeStats(self, request, context):
        if request.volume_id == None or request.volume_id == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id"
            )

        if request.volume_path == None or request.volume_path == '':
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id"
            )

        volume_map = controller.get_volume(request.volume_id)

        if volume_map == None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, "request.target_path does not exits"
            )

        if volume_map.published_path != request.volume_path:
            context.abort(
                grpc.StatusCode.NOT_FOUND, "Invalid volume path"
            )
        usage = []
        usage.append(VolumeUsage(
            available=100, total=10000, used=9900, unit=1))

        condition = VolumeCondition(abnormal=False, message="Ok")

        return NodeGetVolumeStatsResponse(usage=usage, volume_condition=condition)
