#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./lvm_test.yaml

