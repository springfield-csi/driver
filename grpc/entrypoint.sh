#!/bin/bash -x

echo "Starting container for springfield csi...."
echo  container args are: "$@"


set +e
/usr/local/bin/python3 /csi-springfield-driver/server.py "$@"
set -e