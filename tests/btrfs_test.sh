#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./btrfs_test.yaml

