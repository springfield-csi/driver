#!/bin/bash -x

#
# Note: docker builds in /tmp/docker-xxx, so using relative paths
# to reference parent directories to build the image fails.



export CERT_MANAGER_VERSION=v1.7.0
export KIND_VERSION=v0.14.0
export KIND=bin/kind
export KUBECTL=bin/kubectl
export HELM=bin/helm
export BINDIR=./bin
export CLUSTER_NAME=springfield-cluster

export VERSION="0.1.2"


#if pgrep -x "dbus/blivetd" > /dev/null
#then
#   echo "blivetd running"
#else
#   echo "blivetd is required for proper operation of springfield CSI driver"
#   exit 1
#fi

$KUBECTL apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.7.0/cert-manager.crds.yaml
$KUBECTL create namespace springfield-system

$HELM dependency build ./deploy/helm/springfield-csi/
$HELM install --debug --namespace=springfield-system springfield ./deploy/helm/springfield-csi/

echo "Waiting for CSI driver to deploy...."
$KUBECTL wait -l statefulset.kubernetes.io/pod-name=springfield-csi-0 -n springfield-system --for=condition=ready pod --timeout=-100s

if [ $? -eq 0 ]
then
  echo "CSI driver ready."
else
  echo "Driver failed to deploy." >&2
  exit 1
fi

$KUBECTL get storageclass 
