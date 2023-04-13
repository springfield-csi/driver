#!/bin/bash -x

#
# Note: docker builds in /tmp/docker-xxx, so using relative paths
# to reference parent directories to build the image fails.

export CLUSTER_NAME=springfield-cluster
export KUBERNETES_VERSION=v1.24.2
export HELM_VERSION=v3.10.3
export BINDIR=./bin
export TMPDIR=/tmp/springfield
export CERT_MANAGER_VERSION=v1.7.0
export KIND_VERSION=v0.14.0
export KIND=bin/kind
export KUBECTL=bin/kubectl
export HELM=bin/helm
export CURL="curl -sSLf"
export LINVENESS_PROBE=bin/liveness

rm -f bin/springfield-csi-driver:devel.image 

mkdir -p "$SIDECAR_SRC"


if [[ -z "$GITHUB_PAT" ]]; then
    echo "Must provide GITHUB_PAT with write access to ghcr.io/trgill/springfield-csi-driver in environment" 1>&2
    exit 1
fi

if [ ! -d ./bin ]; then
    mkdir -p bin
fi

# Install kind
if [ ! -f "$KIND" ]; then
    $CURL -Lo $KIND https://kind.sigs.k8s.io/dl/$KIND_VERSION/kind-linux-amd64
    chmod +x $KIND
fi

# Install kubectl
if [ ! -f "$KUBECTL" ]; then
    $CURL -Lo $KUBECTL  https://storage.googleapis.com/kubernetes-release/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl 
    chmod 755 $KUBECTL
fi

# Install helm
if [ ! -f "$HELM" ]; then
    $CURL https://get.helm.sh/helm-$HELM_VERSION-linux-amd64.tar.gz \
            | tar xvz -C ./bin --strip-components 1 linux-amd64/helm
fi


docker logout ghcr.io

export VERSION="0.1.0"

# TODO: Add logic to only build the container when necessary.
export TZ="EST"
docker build --no-cache -f deploy/docker/Dockerfile --build-arg  BUILD_DATE="$(date)" -t springfield-csi-driver:devel .

# docker image ls localhost/springfield-csi-driver
echo $GITHUB_PAT | docker login ghcr.io -u trgill --password-stdin

docker tag springfield-csi-driver:devel ghcr.io/trgill/springfield-csi-driver:devel
docker push ghcr.io/trgill/springfield-csi-driver:devel
docker save -o $BINDIR/springfield-csi-driver.image springfield-csi-driver:devel

docker images ghcr.io/trgill/springfield-csi-driver

