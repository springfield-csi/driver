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

import driver
import grpc
from csi_pb2 import (
    GetPluginInfoResponse,
    ProbeResponse,
    GetPluginCapabilitiesResponse,
    PluginCapability,
)
from csi_pb2_grpc import IdentityServicer
import logging

# CSI Spec https://github.com/container-storage-interface/spec/blob/master/spec.md

logger = logging.getLogger("springfield-csi")

class SpringfieldIdentityService(IdentityServicer):
    def GetPluginInfo(self, request, context):
        logger.info("GetPluginInfo()")
        name = driver.DRIVER_NAME
        vendor_version = driver.DRIVER_VERSION

        return GetPluginInfoResponse(name=name, vendor_version=vendor_version)

    def GetPluginCapabilities(self, request, context):
        logger.info("GetPluginCapabilities()")
        control_service = PluginCapability(
            service=PluginCapability.Service(
                type=PluginCapability.Service.CONTROLLER_SERVICE
            )
        )

        topology_service = PluginCapability(
            service=PluginCapability.Service(
                type=PluginCapability.Service.VOLUME_ACCESSIBILITY_CONSTRAINTS
            )
        )

        volume_expansion = PluginCapability(
            volume_expansion=PluginCapability.VolumeExpansion(
                type=PluginCapability.VolumeExpansion.ONLINE
            )
        )

        capabilities = [control_service, volume_expansion, topology_service]

        return GetPluginCapabilitiesResponse(capabilities=capabilities)

    def Probe(self, request, context):
        context.set_code(grpc.StatusCode.OK)
        return ProbeResponse()
