#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./image_test.yaml

