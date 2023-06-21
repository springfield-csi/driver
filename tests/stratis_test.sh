#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./stratis_test.yaml

