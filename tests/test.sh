#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./testpvc.yaml

