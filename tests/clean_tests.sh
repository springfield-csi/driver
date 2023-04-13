#!/bin/bash -x

export KUBECTL=../bin/kubectl
#$KUBECTL delete pv  springfield-pvc-test
$KUBECTL delete pod springfield-pvc-test 
$KUBECTL delete pvc springfield-pvc-claim

