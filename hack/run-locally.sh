#!/bin/bash

# Assuming a running bare metal cluster, ensure the MAO and metal3
# deployments are scaled down before running the CBO with the required
# env vars set

set -x
set -o errexit
set -o nounset

# Apply a CVO overrides to the MAO deployment
oc patch clusterversion version --namespace openshift-cluster-version --type merge \
   -p '{"spec":{"overrides":[{"kind":"Deployment","group":"apps/v1","name":"machine-api-operator","namespace":"openshift-machine-api","unmanaged":true}]}}'

# Scale down the MAO deployment
oc scale -n openshift-machine-api --replicas=0 deployment/machine-api-operator

# Scale down the metal3 deployment
oc -n openshift-machine-api scale --replicas=0 deployment/metal3

# Fetch image pull specs from the MAO images config map
function get_image() {
    image="$1"; shift

    oc get -n "openshift-machine-api" configmap/machine-api-operator-images -o json | jq -r '.data["images.json"]' | jq ".$image"
}

export BAREMETAL_IMAGE=$(get_image baremetalOperator)
export IRONIC_IMAGE=$(get_image baremetalIronic)
export IRONIC_INSPECTOR_IMAGE=$(get_image baremetalIronicInspector)
export IRONIC_IPA_DOWNLOADER_IMAGE=$(get_image baremetalIpaDownloader)
export IRONIC_MACHINE_OS_DOWNLOADER_IMAGE=$(get_image baremetalMachineOsDownloader)
export IRONIC_STATIC_IP_MANAGER_IMAGE=$(get_image baremetalStaticIpManager)

# Get the current release version from the CVO status
export OPERATOR_VERSION=$(oc get clusterversion/version -o json | jq -r .status.desired.version)

export WATCH_NAMESPACE=""
export POD_NAME="cluster-baremetal-operator"
export OPERATOR_NAME="cluster-baremetal-operator"

./build/_output/bin/cluster-baremetal-operator