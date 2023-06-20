#!/bin/bash -x

export KUBECTL=../bin/kubectl

$KUBECTL delete pod springfield-pvc-stratis-test

$KUBECTL delete pvc springfield-pvc-stratis-claim

