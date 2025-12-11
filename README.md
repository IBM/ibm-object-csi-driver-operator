# ibm-object-csi-driver-operator

[![Coverage](https://ibm.github.io/ibm-object-csi-driver-operator/coverage/master/badge.svg)](https://ibm.github.io/ibm-object-csi-driver-operator/coverage/master/cover.html)

A CSI based object storage plugin with dynamic bucket provisioning and plugable mounters, like rclone, s3fs and other. The plugin should seamlessly work in IBM, AWS and GCP platforms.

## Description
ibm-object-csi-driver-operator is a user defined storage controller extending K8S capability to manage COS Buckets as PV/PVC resource.
This is designed and developed as per CSI specification. It enables the application workloads to dynamically provision volumes backed by COS buckets as per PVC/PV framework. As of now it supports s3fs & rclone mounters to mount the bucket as a fuse FileSystem inside the POD. 

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

1. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<image-registry>/ibm-object-csi-driver-operator:<image-tag>
```

2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<image-registry>/ibm-object-csi-driver-operator:<image-tag>
```

3. Install Instances of Custom Resources:

```sh
kubectl apply -k config/samples/
```

**Note**: 
- By default, in the IBM Object CSI Driver, the secret name is not tied to the PVC name. This allows you to use a single secret across multiple PVCs. For this, you’ll need to add two specific annotations in the PVC YAML. These annotations help the driver map the PVC to the correct secret.
```
annotations:
    cos.csi.driver/secret: "custom-secret"
    cos.csi.driver/secret-namespace: "default"
```
- If you want to have 1-to-1 mapping between each PVC and secret(using same name for both) i.e., specific secret tied to respective PVC, then you need to create a custom storage class as below and use it to create PVC.
```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: custom-object-csi-storage-class
  labels:
provisioner: cos.s3.csi.ibm.io
mountOptions:
    - "multipart_size=62"
    - "max_dirty_data=51200"
    - "parallel_count=8"
    - "max_stat_cache_size=100000"
    - "retries=5"
    - "kernel_cache"
parameters:
  mounter: <"s3fs" or "rclone">
  client: "awss3"
  cosEndpoint: "https://s3.direct.us-west.cloud-object-storage.appdomain.cloud"
  locationConstraint: "us-west-smart"
  csi.storage.k8s.io/provisioner-secret-name: ${pvc.name}
  csi.storage.k8s.io/provisioner-secret-namespace: ${pvc.namespace}
  csi.storage.k8s.io/node-publish-secret-name: ${pvc.name}
  csi.storage.k8s.io/node-publish-secret-namespace: ${pvc.namespace}
reclaimPolicy: <Retain or Delete>
```

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

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

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

