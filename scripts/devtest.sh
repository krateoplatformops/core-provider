#!/bin/bash

PROJECT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )/.." &> /dev/null && pwd )
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

cd $PROJECT_DIR

sh ./scripts/kind-up.sh
sh ./scripts/build_local.sh

helm repo update krateo 
helm install --create-namespace --namespace test-system core-provider krateo/core-provider --set image.repository=kind.local/core-provider --set image.tag=latest --set env.CORE_PROVIDER_DEBUG="true"

kubectl config set-context --current --namespace=test-system
kubectl apply -f ./testdata/test 

kubectl config set-context --current --namespace=default