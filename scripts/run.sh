#!/bin/bash


kind get kubeconfig >/dev/null 2>&1 || kind create cluster

export CDC_IMAGE_TAG=0.5.2

kubectl apply -f crds/
kubectl apply -f testdata/ns.yaml
kubectl apply -f testdata/compositiondefinition-postgresql-tgz.yaml
kubectl apply -f testdata/sample.yaml


go run cmd/main.go --debug --poll "3m"