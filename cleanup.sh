#!/bin/bash -x

export KIND=bin/kind
export CLUSTER_NAME=springfield-cluster

$KIND delete cluster --name=$CLUSTER_NAME

# systemctl restart containerd 
systemctl restart podman 

umount /mnt
stratis pool destroy springfield-csi-pool
