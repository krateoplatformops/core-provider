#!/bin/bash

# KO_DOCKER_REPO=kind.local ko build --base-import-paths .  --preserve-import-paths
# KO_DOCKER_REPO=kind.local ko build --base-import-paths .  

KO_DOCKER_REPO=kind.local KIND_CLUSTER_NAME=krateo ko build --base-import-paths .
# KO_DOCKER_REPO=matteogastaldello ko build -t latest --base-import-paths .
printf '\n\nList of current docker images loaded in KinD:\n'

kubectl get nodes kind-control-plane -o json \
    | jq -r '.status.images[] | " - " + .names[-1]'