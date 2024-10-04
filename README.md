# pod-image-aging

The `pod-image-aging` Kubernetes controller tracks the age of your pod's container images and stores the created at
timestamp inside an annotation. This information can be useful to know the age of your pod images and help you to
identify the pods that are running old images.

## Description

The `pod-image-aging` controller watches for pods in your cluster and extracts the image information from the pod's spec
to fetch the created at timestamp from the corresponding container image registry.

Once annotated you can inspect your pod and get the image creation timestamp of all `containers` and `initContainers`
from the `pod-image-aging.hbst.io/status` annotation.

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    pod-image-aging.hbst.io/status: '{"containers":[{"name":"pod-image-aging","createdAt":"2024-09-08T06:45:26Z"}]}'
  name: pod-image-aging-6f6b769dd6-fnplf
  namespace: default
spec:
  containers:
    - image: hebestreit/pod-image-aging:0.0.1
      imagePullPolicy: IfNotPresent
# ...                 
```

If you want to get an overview of all pods and their image creation timestamps, you can pipe the output
of `kubectl get pods -A -o json` to the `hack/format.sh` script:

```shell
kubectl get pods -A -o json | ./hack/format.sh
Fri Oct  4 17:31:15 CEST 2024

NAMESPACE    NAME                                    CONTAINER               IMAGE                                    IMAGE AGE
kube-system  coredns-77ccd57875-4ph22                coredns                 rancher/mirrored-coredns-coredns:1.10.1  86 weeks
kube-system  metrics-server-648b5df564-cz6hm         metrics-server          rancher/mirrored-metrics-server:v0.6.3   80 weeks
kube-system  local-path-provisioner-957fdf8bc-774mc  local-path-provisioner  rancher/local-path-provisioner:v0.0.24   80 weeks
kube-system  traefik-64f55bb67d-qkqtr                traefik                 rancher/mirrored-library-traefik:2.9.10  78 weeks
kube-system  svclb-traefik-6234005d-h72q9            lb-tcp-80               rancher/klipper-lb:v0.4.4                70 weeks
kube-system  svclb-traefik-6234005d-h72q9            lb-tcp-443              rancher/klipper-lb:v0.4.4                70 weeks

NAMESPACE    IMAGE AGE (avg)
kube-system  77 weeks

Overall average: 77.30 weeks
```

## Getting Started

Since the Helm chart is not pushed to a public repository yet, you need to clone the repository:

```shell
git clone git@github.com:hebestreit/pod-image-aging.git
```

### Create a secret for the container registry

In order to fetch the image creation timestamp from private container registries or to prevent running into the
DockerHub rate limit issue, you need to create a secret with the credentials first.

```shell
NAMESPACE="default"
DOCKER_REGISTRY_SERVER="https://index.docker.io/v1/"
DOCKER_USERNAME="docker-username"
DOCKER_PASSWORD="docker-password"
DOCKER_EMAIL="docker-email"

kubectl -n $NAMESPACE create secret docker-registry pod-image-aging-docker-auth \
  --docker-server=DOCKER_REGISTRY_SERVER \
  --docker-username=$DOCKER_USERNAME \
  --docker-password=$DOCKER_PASSWORD \
  --docker-email=$DOCKER_EMAIL
```

If you want to add multiple entries for different registries, you can temporarily store the content of the secret in a
file and create the secret from that file instead:

```shell
NAMESPACE="default"
GITLAB_USERNAME="gitlab-username"
GITLAB_PASSWORD="gitlab-password"
GITLAB_EMAIL="gitlab-email"
GITLAB_AUTH=$(echo -n "$GITLAB_USERNAME:$GITLAB_PASSWORD" | base64)

DOCKER_USERNAME="docker-username"
DOCKER_PASSWORD="docker-password"
DOCKER_EMAIL="docker-email"
DOCKER_AUTH=$(echo -n "$DOCKER_USERNAME:$DOCKER_PASSWORD" | base64)

cat <<EOF > .dockerconfigjson
{
  "auths": {
    "registry.gitlab.com": {
      "username": "$GITLAB_USERNAME",
      "password": "$GITLAB_PASSWORD",
      "email": "$GITLAB_EMAIL",
      "auth": "$GITLAB_AUTH"
    },
    "https://index.docker.io/v1/": {
      "username": "$DOCKER_USERNAME",
      "password": "$DOCKER_PASSWORD",
      "email": "$DOCKER_EMAIL",
      "auth": "$DOCKER_AUTH"
    }
  }
}
EOF
```

Create the secret from the above file:

```shell
kubectl -n $NAMESPACE create secret generic pod-image-aging-docker-auth --type=kubernetes.io/dockerconfigjson --from-file=.dockerconfigjson=.dockerconfigjson
```

### Install using Helm

To install the `pod-image-aging` controller using Helm, you can use the following command and reference the name of the
secret you created in the previous step:

```shell
NAMESPACE="default"
helm upgrade -n $NAMESPACE \
  --install pod-image-aging ./charts/pod-image-aging \
  --set dockerAuthSecretName=pod-image-aging-docker-auth
```

#### Parameters

The following table lists the configurable parameters of the `pod-image-aging` chart and their default values:

| Name                   | Description                                              | Example                                                                                                 | Value                    |
|------------------------|----------------------------------------------------------|---------------------------------------------------------------------------------------------------------|--------------------------|
| `includeNamespaces`    | Comma-separated list of namespaces to include.           | `"kube-system,default"`                                                                                 | `""`                     |
| `excludeNamespaces`    | Comma-separated list of namespaces to exclude.           | `"kube-system,default"`                                                                                 | `""`                     |
| `includeImages`        | Comma-separated list of images to include.               | `"hebestreit/pod-image-aging:*"`                                                                        | `""`                     |
| `excludeImages`        | Comma-separated list of images to exclude.               | `"066635153087.dkr.ecr.il-central-1.amazonaws.com/*,602401143452.dkr.ecr.eu-central-1.amazonaws.com/*"` | `""`                     |
| `cacheExpiry`          | Cache expiry time.                                       | `"168h"`                                                                                                | `"168h"`                 |
| `dockerAuthSecretName` | Name of the secret with the Docker registry credentials. | `"pod-image-aging-docker-auth"`                                                                         | `""`                     |
| `dockerAuthConfigPath` | Path to the Docker config file.                          | `"/.docker/config.json"`                                                                                | `"/.docker/config.json"` |

### Uninstall using Helm

To uninstall the `pod-image-aging` controller, you can use the following command:

```shell
NAMESPACE="default"
helm uninstall -n $NAMESPACE pod-image-aging
```

# Development

Contributions are welcome!

## Getting Started

### Prerequisites

- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/pod-image-aging:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/pod-image-aging:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Contributing

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

