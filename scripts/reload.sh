#!/bin/bash

# CERTIFICATES_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )"/certificates &> /dev/null && pwd )
PROJECT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &> /dev/null && pwd)
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# echo "CERTIFICATES_DIR: ${CERTIFICATES_DIR}"

# cd ${CERTIFICATES_DIR}
# ./create-certs.sh demo-system core-provider-webhook-service

cd ${PROJECT_DIR}

# echo "SCRIPT_DIR: ${SCRIPT_DIR}"

kubectl delete -f manifests/deploy.yaml

${SCRIPT_DIR}/generate.sh

kubectl apply -f crds
kubectl apply -f manifests