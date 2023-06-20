#!/bin/bash -x

export KUBECTL=../bin/kubectl
$KUBECTL apply -f ./md_test.yaml

