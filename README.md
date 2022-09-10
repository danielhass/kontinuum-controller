# kontinuum-controller

> **Please notice:** the Kontinuum-controller is in a Proof-of-Concept stage and demos the functionality of the underlying concept. The project is not yet ready for usage in any kind of production environment.

The Kontinuum-controller projects provides a Kubernetes controller that extends the Kubernetes API to offer three simple Custom Resource Definitions (CRDs) that can be used by application developers to deploy their applications to a variety of clusters may they reside in the cloud, on the edge or basically anywhere. Additionally platform engineers have the possibility to offer managed components (e.g. databases, key-value stores, message-queues etc.) on the composed application platform that can be used by application teams to offload them from operations and management efforts.

Please find a detailed description on the project homepage: [kontinuum-controller.github.io](https://kontinuum-controller.github.io/)

## Installation

### Control Plane Cluster 

1. Clone the git repository and checkout the version of the Kontinuum-Controller you would like to install
2. Generate the Kubernetes manifests (e.g. CRDs):
```
make manifests
```
3. Deploy the Kontinuum-Controller together with the associated RBAC rules
```
make deploy IMG=registry.gitlab.com/danielhass/kontinuum-controller:v0.4.0
```
4. Check the rollout status of the controller manager:
```
kubectl rollout -n continuum-system status deployment/continuum-controller-manager
```

### Storage Layer

The provisioning of the S3 based storage layer is out of scope for the Kontinuum-controller and needs to be taken care of by the user. One or multiple S3 buckets (according to the target landscape and architecture) need to be provisioned on an S3 compatible storage provider.

The Kontinuum-controller maintainers have tested the following providers:
- Amazon Simple Storage Service (S3) - https://aws.amazon.com/s3/
- Digital Ocean Spaces - https://www.digitalocean.com/products/spaces
- Filebase - https://filebase.com/

### Target Clusters

1. For setting up a target cluster your machine needs to have access to the Flux CLI. Please follow the [official Flux docs](https://fluxcd.io/docs/cmd/) on how to install the CLI.
2. Install necessary Flux components on the target cluster:
```
flux install --components="source-controller,kustomize-controller,helm-controller"
```
3. Create namespace for Kontinuum-controller resources:
```
kubectl create ns kontinuum-controller
```
4. Create Flux Helm repository:
```
flux create -n kontinuum-controller source helm endress-continuum --interval=1h --url=<helm_repo_url>
# example
flux create -n kontinuum-controller source helm endress-continuum --interval=1h --url=https://charts.bitnami.com/bitnami
```
5. Create Flux S3 bucket:
```
flux create source bucket aws-demo --bucket-name=<bucket_name> --endpoint=<s3_endpoint> --provider=[generic|aws] --region=<s3_region> --interval=1m --access-key=<s3_access_key> --secret-key=<s3_secret_key>
# example
flux create source bucket aws-demo --bucket-name=my-bucket --endpoint=s3.eu-central-1.amazonaws.com --provider=aws --region=eu-central-1 --interval=1m --access-key=... --secret-key=...
```
6. Create Flux Kustomization:
```
flux create kustomization <bucket_name> --source=Bucket/<bucket_name> --path=<s3_bucket_subfolder> --interval=1m --prune
# example
flux create kustomization aws-demo --source=Bucket/aws-demo --path="./my-target" --interval=1m --prune
```

## Usage

### Targets

Targets are used in the Kontinuum-controller to represent single or multiple target cluster that act as a deployment target on the platform. Create a sample target on the control-plane cluster like this:

1. Create a Kubernetes Secret that holds the S3 credentials for the controller:
```
kubectl create -n continuum-system secret generic s3-secret \
  --from-literal=access-key-id=<s3_access_key> \
  --from-literal=secret-access-key=<s3_secret_key>
```

2. Create a Target object:
```
cat <<EOF | kubectl apply -n continuum-system -f -
apiVersion: crd.kontinuum-controller.github.io/v1alpha1
kind: Target
metadata:
  name: my-target
  labels:
    arch: x86
    group: cloud
    name: my-target
spec:
  s3:
    credentialSecretName: s3-secret
    bucketName: aws-demo
    folder: my-target
    provider: AWS
    endpoint: s3.eu-central-1.amazonaws.com
EOF
```

### Workloads

After a Target has been deployed on the control-plane cluster the users can now start to deploy actual workloads and assign them to different target clusters:

```
cat <<EOF | kubectl apply -n continuum-system -f -
apiVersion: crd.kontinuum-controller.github.io/v1alpha1
kind: Workload
metadata:
  name: workload-sample
spec:
  selector:
    # Target Selector
    matchLabels:
      name: my-target
  components:
  - name: nginx-sample
    # Helm Chart
    type: nginx
    # Helm Values
    values:
      service:
        type: ClusterIP
  managedComponents: []
EOF
```

### Overlays

Overlays can be used by platform operators to provide component defaults, target specific settings and to maintain managed components on the platform. An example for a simple Overlay is given below:

```
cat <<EOF | kubectl apply -n continuum-system -f -
apiVersion: crd.kontinuum-controller.github.io/v1alpha1
kind: Overlay
metadata:
  name: my-overlay
spec:
  selector:
    # Target Selector
    matchLabels:
      group: cloud
  components:
    # Helm Chart
  - type: nginx
    # Helm Values (overwrite/add)
    values:
      podLabels:
        hello: world
  managedComponents: []
EOF
```

## Development

Tooling used on the developers workstation for the current main branch:
```
kubebuilder v3.2.0
go v1.19
```
