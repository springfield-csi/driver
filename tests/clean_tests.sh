#!/bin/bash -x

export KUBECTL=../bin/kubectl

$KUBECTL delete pod springfield-pvc-lvm-test
$KUBECTL delete pod springfield-pvc-btrfs-test
$KUBECTL delete pod springfield-pvc-md-test
$KUBECTL delete pod springfield-pvc-stratis-test

$KUBECTL delete pvc springfield-pvc-lvm-claim
$KUBECTL delete pvc springfield-pvc-btrfs-claim
$KUBECTL delete pvc springfield-pvc-md-claim
$KUBECTL delete pvc springfield-pvc-stratis-claim

