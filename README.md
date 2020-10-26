# Kubernetes Node Auto Labeller

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

depending upon whether we are using the device plugin, NFD-based feature discovery, or DT-based resource discovery.

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

The `labels/` directory (or any other label-containing directory specified) is monitored for changes, such that any
newly created or removed labels or namespaces will be automatically propagated across the cluster at run-time, without
the need to restart the controller.

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
