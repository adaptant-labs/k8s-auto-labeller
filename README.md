# Kubernetes Node Auto Labeller

[![Build Status](https://travis-ci.com/adaptant-labs/k8s-auto-labeller.svg?branch=master)](https://travis-ci.com/adaptant-labs/k8s-auto-labeller)
[![Go Report Card](https://goreportcard.com/badge/github.com/adaptant-labs/k8s-auto-labeller)](https://goreportcard.com/report/github.com/adaptant-labs/k8s-auto-labeller)
[![Docker Pulls](https://img.shields.io/docker/pulls/adaptant/k8s-auto-labeller.svg)](https://hub.docker.com/repository/docker/adaptant/k8s-auto-labeller)

Kubernetes controller for automatically applying node labels when dependent labels are defined.

## Motivation

While many device plugins and resource discovery mechanisms already take care of applying specific labels to individual
nodes, we do not have a mechanism in place for generalizing across these.

As an example, the existence of an EdgeTPU on a node could be expressed through any of:

```
kkohtaka.org/edgetpu
feature.node.kubernetes.io/usb-fe_1a6e_089a.present
feature.node.kubernetes.io/pci-0880_1ac1.present
beta.devicetree.org/fsl-imx8mq-phanbell
```

depending upon whether we are using the device plugin, NFD-based feature discovery, or [DT-based][k8s-dt-node-labeller]
resource discovery.

Deployments wishing to target an EdgeTPU will then have to consider these different scenarios as part of their
`nodeSelectorTerms` when determining node affinity for pod scheduling:

```
spec:
  ...
  template:
  ...
    spec:
      ...
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  # EdgeTPU Device Plugin
                  - key: kkohtaka.org/edgetpu
                    operator: Exists
              - matchExpressions:
                  # USB-attached Coral AI Accelerator (using NFD discovery)
                  - key: feature.node.kubernetes.io/usb-fe_1a6e_089a.present
                    operator: In
                    values:
                      - "true"
              - matchExpressions:
                  # PCIe-attached Coral AI Accelerator (using NFD discovery)
                  - key: feature.node.kubernetes.io/pci-0880_1ac1.present
                    operator: In
                    values:
                      - "true"
              - matchExpressions:
                  # Coral Dev Board (using DT labelling)
                  - key: beta.devicetree.org/fsl-imx8mq-phanbell
                    operator: In
                    values:
                      - "1"
```

This is undesirable for a number of reasons, including:
- Deployments must always be aware of every possible potential label and discovery mechanism.
- As other versions of the accelerator are released, existing deployments need to be updated with the new labels
and are at risk of quickly becoming out of date.

The auto labeller works around this by acting as a single source of truth for generalized labelling, while only
requiring dependent labels to be updated in one place.

[k8s-dt-node-labeller]: https://github.com/adaptant-labs/k8s-dt-node-labeller

## Label Definitions

The auto labeller makes use of a series of flat files that contain dependent labels. These are provided by default in
the `labels/` directory, and follow the format of `labels/<namespace>/<label>`. As an example:

```
$ cat labels/sodalite.eu/edgetpu 
kkohtaka.org/edgetpu
feature.node.kubernetes.io/usb-fe_1a6e_089a.present
feature.node.kubernetes.io/pci-0880_1ac1.present
beta.devicetree.org/fsl-imx8mq-phanbell
```

will automatically apply the `sodalite.eu/edgetpu` label to any nodes that contain any of the labels in the label file.

The `labels/` directory (or any other label-containing directory specified with `-label-dir`) is monitored for changes,
such that any newly created or removed labels or namespaces will be automatically propagated across the cluster at
run-time, without the need to restart the controller.

### Label Volumes

If planning to use an external volume for label definitions, these can be passed in to the container via a Docker volume
under the `/labels` mount point. Note that the contents of the `labels/` directory will already be exposed as a
persisted volume when the container is first run.

## Usage

General usage is as follows:

```
$ k8s-auto-labeller --help
Node Auto Labeller for Kubernetes
Usage: ./k8s-auto-labeller [flags]
  -kubeconfig string
	Paths to a kubeconfig. Only required if out-of-cluster.
  -label-dir string
	Label directory to monitor (default "labels")
  -master --kubeconfig
	(Deprecated: switch to --kubeconfig) The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.
```

Using a USB-attached EdgeTPU as an example:

```
$ k8s-auto-labeller
{"level":"info","ts":1604007914.093528,"logger":"k8s-auto-labeller","msg":"Adding watcher","dir":"/labels"}
{"level":"info","ts":1604007914.0936112,"logger":"k8s-auto-labeller","msg":"Adding watcher","dir":"/labels/sodalite.eu"}
{"level":"info","ts":1604007914.5515614,"logger":"controller-runtime.metrics","msg":"metrics server is starting to listen","addr":":8080"}
{"level":"info","ts":1604007914.5517762,"logger":"k8s-auto-labeller.watcher","msg":"Monitoring filesystem for events..."}
{"level":"info","ts":1604007914.5517755,"logger":"k8s-auto-labeller.entrypoint","msg":"starting manager"}
{"level":"info","ts":1604007914.5519438,"logger":"controller-runtime.manager","msg":"starting metrics server","path":"/metrics"}
{"level":"info","ts":1604007914.551965,"logger":"controller","msg":"Starting EventSource","controller":"k8s-auto-labeller","source":"kind source: /, Kind="}
{"level":"info","ts":1604007914.6523216,"logger":"controller","msg":"Starting Controller","controller":"k8s-auto-labeller"}
{"level":"info","ts":1604007914.65236,"logger":"controller","msg":"Starting workers","controller":"k8s-auto-labeller","worker count":1}
{"level":"info","ts":1604007914.6525555,"logger":"k8s-auto-labeller.reconciler","msg":"Reconciling node","request":"/sgx-celsius-w550power","node":"sgx-celsius-w550power"}
{"level":"info","ts":1604007914.6526318,"logger":"k8s-auto-labeller.reconciler","msg":"Setting label","request":"/sgx-celsius-w550power","label":"sodalite.eu/edgetpu"}
...
```

The label state can be confirmed by querying the node labels directly:

```
$ kubectl get nodes sgx-celsius-w550power --show-labels
NAME                    STATUS   ROLES    AGE    VERSION        LABELS
sgx-celsius-w550power   Ready    <none>   148d   v1.18.3+k3s1   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/instance-type=k3s,beta.kubernetes.io/os=linux,feature.node.kubernetes.io/usb-fe_1a6e_089a.present=true,k3s.io/hostname=sgx-celsius-w550power,k3s.io/internal-ip=192.168.188.92,kubernetes.io/arch=amd64,kubernetes.io/hostname=sgx-celsius-w550power,kubernetes.io/os=linux,node.kubernetes.io/instance-type=k3s,sodalite.eu/edgetpu=true
```

when the device is removed, the label is automatically cleared:

```
{"level":"info","ts":1604008598.1130025,"logger":"k8s-auto-labeller.reconciler","msg":"Reconciling node","request":"/sgx-celsius-w550power","node":"sgx-celsius-w550power"}
{"level":"info","ts":1604008598.1130307,"logger":"k8s-auto-labeller.reconciler","msg":"Clearing label","request":"/sgx-celsius-w550power","label":"sodalite.eu/edgetpu"}
...

$ kubectl get nodes sgx-celsius-w550power --show-labels
NAME                    STATUS   ROLES    AGE    VERSION        LABELS
sgx-celsius-w550power   Ready    <none>   148d   v1.18.3+k3s1   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/instance-type=k3s,beta.kubernetes.io/os=linux,k3s.io/hostname=sgx-celsius-w550power,k3s.io/internal-ip=192.168.188.92,kubernetes.io/arch=amd64,kubernetes.io/hostname=sgx-celsius-w550power,kubernetes.io/os=linux,node.kubernetes.io/instance-type=k3s
```

### Running as a Kubernetes Deployment

An example Deployment configuration for automated cluster-wide node labelling is provided in
`k8s-auto-labeller-deployment.yaml`, which can be, as the name implies, directly applied to the running cluster:

```
$ kubectl apply -f https://raw.githubusercontent.com/adaptant-labs/k8s-auto-labeller/k8s-auto-labeller-deployment.yaml
```

This will create a single Deployment in the cluster under the `kube-system` namespace. It will further create a special
`auto-labeller` service account, cluster role, and binding with the permission to list, watch, and update nodes.

Multi-arch containers are provided, allowing for direct deployment into clusters with both `amd64` and `arm64` nodes.

### Automated Descheduling of Pods

When used in conjunction with the [k8s-node-label-monitor], it is possible to automatically deschedule pods that have been
scheduled with label dependencies that are no longer satisfied. This can be useful for evicting pods that have a hard
hardware dependency where it no longer makes sense to keep them running when the hardware is no longer available.

[k8s-node-label-monitor]: https://github.com/adaptant-labs/k8s-node-label-monitor

## Features and bugs

Please file feature requests and bugs in the [issue tracker][tracker].

## Acknowledgements

This project has received funding from the European Union’s Horizon 2020 research and innovation programme under grant
agreement No 825480 ([SODALITE]).

## License

`k8s-auto-labeller` is licensed under the terms of the Apache 2.0 license, the full
version of which can be found in the LICENSE file included in the distribution.

[tracker]: https://github.com/adaptant-labs/k8s-auto-labeller/issues
[SODALITE]: https://www.sodalite.eu
