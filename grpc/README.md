
# Overview

Prototype CSI driver back by Stratis.

Stratis provides high level APIs to Linux device mapper thin provisioned
pools.

This is a prototype of a CSI driver over the Stratis DBUS API.

# To generate the grpc code
cd grpc
python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. csi.proto

# Dependencies

dnf install dbus-devel glib2-devel

python3 -m pip install --upgrade pip
python3 -m pip install install dbus-python protobuf grpcio grpcio-tools