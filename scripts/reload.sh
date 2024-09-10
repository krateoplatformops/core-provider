#!/bin/bash

PROJECT_DIR=$( pwd )
CERTIFICATES_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )"/certificates &> /dev/null && pwd )
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "CERTIFICATES_DIR: ${CERTIFICATES_DIR}"

cd ${CERTIFICATES_DIR}
./create-certs.sh default core-provider-webhook-service

cd ${PROJECT_DIR}

# SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
echo "SCRIPT_DIR: ${SCRIPT_DIR}"

kubectl delete -f manifests/deploy.yaml

${SCRIPT_DIR}/generate.sh
${SCRIPT_DIR}/build.sh

kubectl apply -f crds
kubectl apply -f manifests