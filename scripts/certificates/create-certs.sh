#! /bin/bash

WEBHOOK_NS=${1:-"default"}
WEBHOOK_NAME=${2:-"webhook-service"}


# Create certs for our webhook
openssl genrsa -out tls.key 2048
openssl req -new -key ./tls.key \
    -subj "/CN=${WEBHOOK_NAME}.${WEBHOOK_NS}.svc" \
    -addext "subjectAltName = DNS:${WEBHOOK_NAME}.${WEBHOOK_NS}.svc" \
    -out ./tls.csr \
    -config ./csr.conf
openssl x509 -req -extfile <(printf "subjectAltName=DNS:${WEBHOOK_NAME}.${WEBHOOK_NS}.svc,DNS:${WEBHOOK_NAME}.${WEBHOOK_NS}.svc\nbasicConstraints=CA:TRUE\n") -days 365 -in tls.csr -signkey tls.key -out tls.crt

#openssl x509 -noout -text -in ./webhook.crt 

# # Create certs secrets for k8s
kubectl create secret generic \
    ${WEBHOOK_NAME}-certs \
    --from-file=tls.key=./tls.key \
    --from-file=tls.crt=./tls.crt \
    --dry-run=client -o yaml > ./webhook-certs.yaml\
    -n demo-system


# # Set the CABundle on the webhook registration
CA_BUNDLE=$(cat ./tls.crt | base64 -b 0)
sed "s/CA_BUNDLE/${CA_BUNDLE}/" ./mutate.yaml.tpl > ./mutate.yaml

# ## For conversion test 
# sed "s/CA_BUNDLE/${CA_BUNDLE}/" ./patch.yaml.tpl > ./patch.yaml


kubectl apply -f ./mutate.yaml
kubectl apply -f ./webhook-certs.yaml

rm ./webhook-certs.yaml
# rm ./mutate.yaml
rm ./tls.*