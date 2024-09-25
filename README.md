# pod-image-aging

The `pod-image-aging` Kubernetes controller tracks the age of your pod's container images and stores the created at timestamp inside an annotation. 
This information can be useful to track the age of the images in your pods and help you to identify the pods that are running old images.

## Description

The `pod-image-aging` controller watches for pods in your cluster and extracts the image information from the pod's spec to fetch the created at timestamp from the corresponding container image registry.

Once annotated you can inspect your pod and get the image creation timestamp of all `containers` and `initContainers` from the `pod-image-aging.hbst.io/status` annotation.

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

If you want to get an overview of all pods and their image creation timestamps, you can execute the following command using `kubectl` and `jq`:

```
$  kubectl get pods -A -o json | jq '[. | {pod: .items[].metadata}][] | {name: .pod.name, namespace: .pod.namespace, images: .pod.annotations["pod-image-aging.hbst.io/status"] | fromjson}'

{
  "name": "metrics-server-648b5df564-fz9hd",
  "namespace": "kube-system",
  "images": {
    "containers": [
      {
        "name": "metrics-server",
        "createdAt": "2023-03-21T13:21:03.301449922Z"
      }
    ]
  }
}
{
  "name": "traefik-64f55bb67d-n548j",
  "namespace": "kube-system",
  "images": {
    "containers": [
      {
        "name": "traefik",
        "createdAt": "2023-04-06T18:51:04.986454508Z"
      }
    ]
  }
}
{
  "name": "pod-image-aging-6f6b769dd6-fnplf",
  "namespace": "default",
  "images": {
    "containers": [
      {
        "name": "pod-image-aging",
        "createdAt": "2024-09-08T06:45:26Z"
      }
    ]
  }
}
```

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
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/pod-image-aging:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/pod-image-aging/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

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

