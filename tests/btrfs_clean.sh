#!/bin/bash -x

export KUBECTL=../bin/kubectl

$KUBECTL delete pod springfield-pvc-btrfs-test

$KUBECTL delete pvc springfield-btrfs-pvc-claim 

