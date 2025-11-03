#!/bin/bash

KO_DOCKER_REPO=kind.local KIND_CLUSTER_NAME=krateo-v2 ko build --base-import-paths ..
printf '\n\nList of current docker images loaded in KinD:\n'

kubectl get nodes kind-control-plane -o json \
    | jq -r '.status.images[] | " - " + .names[-1]'