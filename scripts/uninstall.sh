#!/bin/bash -x

export HELM=bin/helm

$HELM uninstall --debug --namespace=springfield-system springfield

