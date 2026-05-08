#!/bin/bash -x

export KUBERNETES_VERSION=v1.24.2
export HELM_VERSION=v3.10.3
export BINDIR=./bin
export KIND_VERSION=v0.14.0
export KIND=bin/kind
export KUBECTL=bin/kubectl
export CURL="curl -sSLf"


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
    $CURL -Lo $KUBECTL https://storage.googleapis.com/kubernetes-release/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl 
    chmod 755 $KUBECTL
fi

# Install helm
if [ ! -f "$HELM" ]; then
    $CURL https://get.helm.sh/helm-$HELM_VERSION-linux-amd64.tar.gz \
            | tar xvz -C ./bin --strip-components 1 linux-amd64/helm
fi

dnf -y install kubernetes-client
dnf -y install stratisd
dnf -y install stratis-cli
dnf -y install podman
