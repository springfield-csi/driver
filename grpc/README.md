
# Overview

Blivet provides high level APIs to most Linux storage technologies.

This is a prototype of a CSI driver over the Blivet API.

Currently just a LVM pool is used, but future enhancements will add more
features.

# To generate the grpc code
cd grpc
python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. api.proto

# Dependencies
pip3 install blivet
pip3 install vext
pip3 install vext.gi
pip3 install gobject
dnf install python3-gi
