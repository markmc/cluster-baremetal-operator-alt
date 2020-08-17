FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/cluster-baremetal-operator \
    USER_UID=1001 \
    USER_NAME=cluster-baremetal-operator

# install operator binary
COPY build/_output/bin/cluster-baremetal-operator ${OPERATOR}

# install manifests
COPY manifests/*.yaml /manifests/
COPY manifests/image-references /manifests/
LABEL io.openshift.release.operator true

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
