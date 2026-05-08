#!/bin/bash -x

export CLUSTER_NAME=springfield-cluster
export KIND=bin/kind

$KIND delete cluster --name=$CLUSTER_NAME

$KIND create cluster --name=$CLUSTER_NAME --wait 2m --config tests/kind/springfield-cluster.yaml  

