#!/bin/bash

# Assuming a running bare metal cluster, ensure the MAO deployment
# is scaled down before creating the CBO deployment using the image
# from quay.io

set -x
set -o errexit
set -o nounset

source ./hack/runutils.sh

scale_down_mao

set_operator_env

OPERATOR_IMAGE="quay.io/markmc/cluster-baremetal-operator:v0.0.1"

# Resolve operator image to a digest form
OPERATOR_DIGEST=$(oc image info "$OPERATOR_IMAGE" -o json | jq -r .digest)
OPERATOR_IMAGE=$(echo -n "${OPERATOR_IMAGE}" | sed "s|:.*|@${OPERATOR_DIGEST}|")

# Mirror operator image to openshift-metal3/dev-scripts local image mirror
# This is required in an ipv6 dev env
if [ -d ~/git/openshift-metal3/dev-scripts ]; then
    source ~/git/openshift-metal3/dev-scripts/common.sh
    if [ ! -z "${MIRROR_IMAGES}" ]; then
        OPERATOR_IMAGE_MIRROR="${LOCAL_REGISTRY_DNS_NAME}:${LOCAL_REGISTRY_PORT}/localimages/cluster-baremetal-operator"
        oc image mirror -a "${REGISTRY_CREDS}" "${OPERATOR_IMAGE}" "${OPERATOR_IMAGE_MIRROR}"
        OPERATOR_IMAGE="${OPERATOR_IMAGE_MIRROR}@${OPERATOR_DIGEST}"
    fi
fi

MANIFESTS_TMPDIR=$(mktemp --tmpdir -d "cbo-manifests-XXXXXXXXXX")
trap "rm -rf ${MANIFESTS_TMPDIR}" EXIT

cp ./manifests/0000_30_cluster-baremetal-operator_05_deployment.yaml ${MANIFESTS_TMPDIR}/05_deployment.yaml

sed -i \
    -e "s|0.0.1-snapshot|${OPERATOR_VERSION}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:cluster-baremetal-operator|${OPERATOR_IMAGE}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:baremetal-operator|${BAREMETAL_IMAGE}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:ironic\([^-]\)|${IRONIC_IMAGE}\1|" \
    -e "s|registry.svc.ci.openshift.org/openshift:ironic-inspector|${IRONIC_INSPECTOR_IMAGE}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:ironic-ipa-downloader|${IRONIC_IPA_DOWNLOADER_IMAGE}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:ironic-machine-os-downloader|${IRONIC_MACHINE_OS_DOWNLOADER_IMAGE}|" \
    -e "s|registry.svc.ci.openshift.org/openshift:ironic-static-ip-manager|${IRONIC_STATIC_IP_MANAGER_IMAGE}|" \
    ${MANIFESTS_TMPDIR}/05_deployment.yaml

diff -u ./manifests/0000_30_cluster-baremetal-operator_05_deployment.yaml ${MANIFESTS_TMPDIR}/05_deployment.yaml >&2 || true

oc apply -f ${MANIFESTS_TMPDIR}/05_deployment.yaml
