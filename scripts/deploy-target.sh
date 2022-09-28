#!/bin/bash

# kontinuum-controller target deploy script
# needs to be called from repository root

# variables
K8S_NAMESPACE=kontinuum-controller

# check if Flux CLI is installed
if ! command -v flux &>/dev/null; then
    echo "ERROR: flux is not installed; please visit https://fluxcd.io/flux/cmd/" 1>&2
    exit 1
fi

echo "> We are going to deploy Flux to the following cluster:"
kubectl cluster-info
sleep 5

echo "> Install necessary Flux components on the target cluster"
flux install --components="source-controller,kustomize-controller,helm-controller"

echo "> Create namespace for kontinuum-controller resources"
kubectl create ns ${K8S_NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

echo -n "> Please enter the Helm repository URL: "
read HELM_URL

echo "> Creating Flux Helm source on target cluster: "
flux create -n ${K8S_NAMESPACE} source helm kontinuum-catalog --interval=1h --url=${HELM_URL}

echo -n "> Please enter the S3 endpoint URL: "
read S3_URL

echo -n "> Please enter the S3 bucket name: "
read S3_BUCKET

echo -n "> Please enter the S3 access key: "
read S3_ACCESS_KEY

echo -n "> Please enter the S3 secret key: "
read S3_SECRET_KEY

echo "> Create Flux S3 bucket reference on target cluster: "
flux create -n ${K8S_NAMESPACE} source bucket ${S3_BUCKET} --bucket-name=${S3_BUCKET} --endpoint=${S3_URL} --provider=generic --interval=1m --access-key=${S3_ACCESS_KEY} --secret-key=${S3_SECRET_KEY}

echo -n "> Please enter the new target cluster name (used to reference it on the control-plane): "
read TARGET_NAME

echo "> Create Flux S3 bucket reference on target cluster: "
flux create -n ${K8S_NAMESPACE} kustomization ${S3_BUCKET} --source=Bucket/${S3_BUCKET} --path="./${TARGET_NAME}" --interval=1m --prune

if [ "$?" -ne 0 ]; then
    echo "> The last step might display an error 'kustomization path not found'. 
        However the resource is correctly applied to the cluster and will start reconciling correctly
        as soon as the corresponding Target object is applied on the control-plane cluster."
fi
