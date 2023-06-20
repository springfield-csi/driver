#!/bin/bash -x

export KUBECTL=../bin/kubectl

$KUBECTL delete pod springfield-pvc-md-test

$KUBECTL delete pvc springfield-pvc-md-claim

