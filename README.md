# deployment-operator
Simple Kubernetes operator to deploy your application.

## Description
The `deployment-operator` can take care of all the resources and configurations that are needed in order to properly operate your application in a Kubernetes cluster.
It configures, creates and monitors a Deployment, a Service and an Ingress. All you need to do is install the required Custom Resource Definitions, deploy the operator and create your Custom Resource. After that you will be able to access your application on the defined Host address with HTTPS.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

## Installation

### Prerequisites

- **cert-manager** to manage certificate related tasks. See the official documentation how to install it (https://cert-manager.io/docs/installation/) or if you don't want to configure anything, just install with the official configs: `kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.11.0/cert-manager.yaml`

- **CertificateIssuer** to issue certificates for your cluster. After installing `cert-manager` your cluster will have a `CertificateIssuer` CRD. Create a Custom Resource of this type: if you have a valid public domain you can use Let's encrypt (https://cert-manager.io/docs/configuration/acme/http01/) for local development use a self signed certificate (https://cert-manager.io/docs/configuration/selfsigned/). There are sample configurations for both in the `config/samples/cert` folder. 

- **NGINX Ingress controller** to control ingresses and route trafic from outside of the cluster. See the official documentation how to install it (https://kubernetes.github.io/ingress-nginx/deploy/). Since we mentioined KIND for local development, if you are using KIND check out this Ingress controller installation guide: https://kind.sigs.k8s.io/docs/user/ingress/#ingress-nginx

### Deploy the operator
1. Install the AppDeployer Custom Resource Definition: 

```sh
make install
```

2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=ghcr.io/rappizs/deployment-operator:latest
```

3. Create an AppDeployer Custom Resource in your cluster: you can find an example in `config/samples`. Make sure that your cluster has every prerequisite and change the clusterIssuer field to the name of your ClusterIssuer CR, if needed. The AppDeployer type has the following spec structure:

```
apiVersion: deployer.rappizs.com/v1
kind: AppDeployer
...
spec:
  replicas: 4 --- replica count for the Deployment
  host: local.nginx.com --- your host address
  image: ghcr.io/rappizs/nginx-hello:latest --- image to deploy
  containerPort: 80 --- port to expose on the containers
  servicePort: 80 --- port of the service
  clusterIssuer: appdeployer-issuer --- name of your ClusterIssuer
```

4. That's it, you should be able to access your application on the configured host address using HTTPS. If you have HTTPS problems with the certifications, check the configuration of cert-manager and your ClusterIssuer. Happy Hacking!

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.


