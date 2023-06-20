#!/bin/bash -x

export KUBECTL=../bin/kubectl

$KUBECTL delete pod springfield-pvc-lvm-test

$KUBECTL delete pvc springfield-pvc-lvm-claim

