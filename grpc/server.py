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

import logging
import sys

file_handler = logging.FileHandler(filename="/tmp/csi_driver.log")
stdout_handler = logging.StreamHandler(stream=sys.stdout)
handlers = [file_handler, stdout_handler]

logging.basicConfig(
    level=logging.DEBUG,
    format="[%(asctime)s] {%(filename)s:%(lineno)d} %(levelname)s - %(message)s",
    handlers=handlers,
)

logger = logging.getLogger("springfield-csi")

import concurrent.futures as futures
import csi_pb2_grpc
import grpc

import argparse
import json
import socket

from pathlib import Path

from node import SpringfieldNodeService
from identity import SpringfieldIdentityService
from controller import SpringfieldControllerService



def initilize_disks(storage_devs):

    logger.info("initilize_disks")
    # path = Path(STORAGE_DEVS_FILE)

    # if not path.is_file():
    #     logger.error('%s file not found in %s',
    #                     STORAGE_DEVS_FILE, Path.cwd())
        
    #     # look in the grpc subdirectory
    #     path = Path("grpc/" + STORAGE_DEVS_FILE)
    #     if not path.is_file():
    #         logger.error('%s file not found in %s',
    #                       STORAGE_DEVS_FILE, Path.cwd())
    #         exit()

    # try:
    #     with open(path) as json_file:
    #         storage_devs = json.load(json_file)['use_for_csi_storage']
    # except ValueError:
    #     logger.error('Failed to parse {}', STORAGE_DEVS_FILE)
    #     exit()

    # pool_path = pool_object_path(CONTAINER_POOL)

    # if pool_path is None:
    #     (result, rc, msg) = pool_create(CONTAINER_POOL, storage_devs)
    #     if (rc != 0): 
    #         logger.error("Failed to initialize stratis pool: " + CONTAINER_POOL + " - " + msg)
    #         exit()
        


def run_server(port, addr, nodeid):
    logger.info("Starting grpc server:")

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    csi_pb2_grpc.add_ControllerServicer_to_server(
        SpringfieldControllerService(nodeid=nodeid), server
    )
    csi_pb2_grpc.add_IdentityServicer_to_server(SpringfieldIdentityService(), server)
    csi_pb2_grpc.add_NodeServicer_to_server(
        SpringfieldNodeService(nodeid=nodeid), server
    )

    server.add_insecure_port("unix://csi/csi.sock")
    # server.add_insecure_port("[::]:9080")
    server.start()
    server.wait_for_termination()

class split_args(argparse.Action):
    def __call__(self, parser, namespace, values, option_string=None):
        setattr(namespace, self.dest, values.split(' '))

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--nodeid",
        dest="nodeid",
        type=str,
        help="unique node identifier",
        default=socket.gethostname(),
    )
    parser.add_argument(
        "--addr", dest="addr", type=str, help="ip address to listen", default="[::]:"
    )
    parser.add_argument(
        "--port", dest="port", type=int, help="port to listen", default=50024
    )
    parser.add_argument(
        "--nodeonly", dest="nodeonly", action=argparse.BooleanOptionalAction
    )

    args = parser.parse_args()

    port = args.port
    addr = args.addr
    nodeid = args.nodeid

    logger.info(sys.argv)

    if not args.nodeonly:
        logger.info("Running in controller mode")
    else:
        logger.info("Running in node mode")

    logger.info("node id = %s", nodeid)
    run_server(port, addr, nodeid)
