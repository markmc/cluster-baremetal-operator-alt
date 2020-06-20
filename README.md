# Cluster BareMetal Operator (CBO)

This is a work-in-progress Second Level Operator (SLO) for bare metal.

See [OpenShift enhancement proposal #212](https://github.com/openshift/enhancements/pull/212) where the design is being discussed

## Hacking

To hack on his operator, you should first deploy a bare metal cluster using [openshift-metal3/dev-scripts](https://github.com/openshift-metal3/dev-scripts).

Then build and push an image to your private quay.io repo:

```
$> make image push
Building image quay.io/markmc/cluster-baremetal-operator:v0.0.1...
...
Pushing images...
...
```

Finally, use the `hack/run-image.sh` script to scale down the Machine API Operator (MAO), and create a CBO deployment with your newly built image:

```
$> hack/run-image.sh
```

## TODO

This work is in its very early stages, so this is just a rough TODO list for now.

Short term:

- Allow `make image push` and `hack/run-image.sh` use other quay repos than `markmc/cluster-baremetal-operator`
- Build a release image which includes CBO and test that it works as expected
- Update the enhancement with latest thinking and findings - e.g. continuing to use the openshift-machine-api namespace
- Implement correct ClusterOperator semantics
- Figure out an in-place upgrade plan
- Implement "disabled SLO" behavior on non-baremetal platforms
- Unit tests - what have we got, what are we missing?
- E2E tests

Exiting prototype phase:

- Move this repo into the openshift org
- Add [https://github.com/openshift/release](openshift/release) config for building and publishing images

MAO changes to pick up:

- openshift/machine-api-operator#498 - hardware inventory management DaemonSet
- openshift/machine-api-operator#547 - podman support

Longer-term or lower priority:

- Move in-memory metal3 deployment resource generation tracking to a CR with OperatorStatus
- Metrics, alerts, events - what have we got? what are we missing?
- RBAC audit - do our roles have the minimal set of permissions?
