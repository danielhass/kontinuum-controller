#!/bin/bash

# kontinuum-controller deploy script
# needs to be called from repository root

echo "> We are going to deploy the kontinuum-controller to the following cluster:"
kubectl cluster-info
sleep 5

echo "> Generating installation manifests"
make manifests

echo "> Depolying the controller"
make deploy IMG=registry.gitlab.com/danielhass/kontinuum-controller:v0.4.0

echo "> Displaying rollout status"
kubectl rollout -n kontinuum-controller-system status deployment/kontinuum-controller-controller-manager
