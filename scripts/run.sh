#!/bin/bash

export CDC_IMAGE_TAG=0.5.2

kubectl apply -f crds/
kubectl apply -f testdata/ns.yaml
kubectl apply -f testdata/definition-postgresql-tgz.yaml


go run cmd/main.go --debug --poll "3m"