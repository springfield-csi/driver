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

from controller import logger
from google.protobuf.json_format import MessageToJson, MessageToDict
from csi_pb2_grpc import NodeServicer

import csi_pb2
from csi_pb2 import (
    NodeGetCapabilitiesResponse,
    NodeGetInfoResponse,
    NodePublishVolumeResponse,
    NodeGetVolumeStatsResponse,
    NodeExpandVolumeResponse,
    NodeServiceCapability,
    NodeStageVolumeResponse,
    NodeUnstageVolumeResponse,
    NodeUnpublishVolumeResponse,
    Topology,
    VolumeUsage,
    VolumeCondition,
)

import grpc

import controller
from stratis import STRATIS_PATH, CONTAINER_POOL

import os
from sh import mount, umount


# CSI Spec https://github.com/container-storage-interface/spec/blob/master/spec.md


class SpringfieldNodeService(NodeServicer):
    def __init__(self, nodeid):
        self.nodeid = nodeid

    def NodeStageVolume(self, request, context):
        logger.info("NodeStageVolume()")
        
        staging_target_path = request.staging_target_path
        exists = os.path.exists(staging_target_path)
        if not exists:
            os.makedirs(staging_target_path)

        stratis_dev_path = STRATIS_PATH + "/" + CONTAINER_POOL + "/"

        logger.info("mount :" + stratis_dev_path + request.volume_id + " on: " + staging_target_path + " with : -txfs")
        try:
            mount(stratis_dev_path + request.volume_id, staging_target_path, "-txfs")
        except OSError as e:
          logger.warning(
                "Warining mount failed: %s : %s" % (staging_target_path, e.strerror)
            )

        return NodeStageVolumeResponse()
    
    def NodeUnstageVolume(self, request, context):
        logger.info("NodeUnstageVolume()")
        # Destroy stratis FS

        staging_target_path = request.staging_target_path

        if not os.path.ismount(staging_target_path):
            logger.warning('NodeUnpublishVolume: {} is already un-mounted'.format(staging_target_path))
        else:
            umount(staging_target_path)

        if os.path.isdir(staging_target_path):
            logger.debug('NodeUnstageVolume removing stage dir: {}'.format(staging_target_path))
            os.rmdir(staging_target_path)

        return NodeUnstageVolumeResponse()
    
    def NodePublishVolume(self, request, context):
        logger.info("NodePublishVolume()")
        if request.volume_id == None or request.volume_id == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id")

        if request.target_path == None or request.target_path == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include target_path")

        staging_target_path = request.staging_target_path
        publish_path = request.target_path

        logger.info("NodePublishVolume(): staging_target_path = %s, publish_path = %s", staging_target_path, publish_path)
        
        volume_capability = request.volume_capability
        
        fstype = volume_capability.mount.fs_type

        # access_type = volume_capability.WhichOneof("access_type")
        # assert access_type == "mount" or access_type == "block"

        # if fstype not in ["xfs"]:
        #     context.abort(
        #         grpc.StatusCode.INVALID_ARGUMENT,
        #         "Unsupported filesystem type: {fstype}",
        #     )


        if volume_capability.access_mode.mode not in [
            csi_pb2.VolumeCapability.AccessMode.Mode.SINGLE_NODE_WRITER
        ]:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                "Unsupported access mode: {csi_pb2.VolumeCapability.AccessMode.Mode.Name(volume_capability.access_mode.mode)}",
            )

        exists = os.path.exists(publish_path)
        if not exists:
            os.makedirs(publish_path)


        logger.info("mount :" + staging_target_path + " on: " + publish_path + " with : --bind")
        try:
            mount(staging_target_path, publish_path, "--bind")
        except OSError as e:
          logger.warning(
                "Warining mount failed: %s : %s" % (staging_target_path, e.strerror)
            )

        return NodePublishVolumeResponse()

    def NodeUnpublishVolume(self, request, context):
        logger.info("NodeUnpublishVolume()")

        if request.volume_id == None or request.volume_id == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id")

        if request.target_path == None or request.target_path == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include target_path")
        if not os.path.exists(request.target_path):
            context.abort(
                grpc.StatusCode.NOT_FOUND, "request.target_path does not exits"
            )
        volume_id = request.volume_id
        target_path = request.target_path

        if not os.path.ismount(target_path):
            logger.warning('NodeUnpublishVolume: {} is already un-mounted'.format(target_path))
        else:
            umount(target_path)

        return NodeUnpublishVolumeResponse()

    def NodeGetCapabilities(self, request, context):
        logger.info("NodeGetCapabilities()")
        get_volume_stats = NodeServiceCapability(
            rpc=NodeServiceCapability.RPC(
                type=NodeServiceCapability.RPC.GET_VOLUME_STATS
            )
        )
        stage_unstage = NodeServiceCapability(
            rpc=NodeServiceCapability.RPC(
                type=NodeServiceCapability.RPC.STAGE_UNSTAGE_VOLUME
            )
        )
        expand_volume = NodeServiceCapability(
            rpc=NodeServiceCapability.RPC(type=NodeServiceCapability.RPC.EXPAND_VOLUME)
        )

        capabilities = [get_volume_stats, stage_unstage, expand_volume]
        return NodeGetCapabilitiesResponse(capabilities=capabilities)

    def NodeGetInfo(self, request, context):
        logger.info("NodeGetInfo() nodeid = %s", self.nodeid)
        return NodeGetInfoResponse(
            node_id=self.nodeid,
            accessible_topology=Topology(segments={"hostname": self.nodeid}),
        )

    def NodeExpandVolume(self, request, context):
        logger.info("NodeExpandVolume() called, which is not implemented")

        return NodeExpandVolumeResponse()

    def NodeGetVolumeStats(self, request, context):
        logger.info("NodeGetVolumeStats()")
        if request.volume_id == None or request.volume_id == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id")

        if request.volume_path == None or request.volume_path == "":
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Must include volume_id")

        volume_map = controller.get_volume(request.volume_id)

        if volume_map == None:
            context.abort(
                grpc.StatusCode.NOT_FOUND, "request.target_path does not exits"
            )

        if volume_map.published_path != request.volume_path:
            context.abort(grpc.StatusCode.NOT_FOUND, "Invalid volume path")
        usage = []
        usage.append(VolumeUsage(available=100, total=10000, used=9900, unit=1))

        condition = VolumeCondition(abnormal=False, message="Ok")

        return NodeGetVolumeStatsResponse(usage=usage, volume_condition=condition)
