ARG BASE_OS_IMAGE
FROM ${BASE_OS_IMAGE}

ARG RKE2_VERSION=v1.21.2+rke2r1
ARG RKE2_IMAGES_URL=https://github.com/rancher/rke2/releases/download/${RKE2_VERSION}
ARG IMAGES_DIR=/var/lib/harvester/images

# packages
RUN zypper in -y wget zstd

# RKE2
RUN curl -sfL https://get.rke2.io --output /tmp/install.sh && chmod +x /tmp/install.sh && sed -i '/systemctl daemon-reload/d' /tmp/install.sh
RUN INSTALL_RKE2_METHOD="tar" INSTALL_RKE2_TAR_PREFIX="/usr" INSTALL_RKE2_VERSION="$RKE2_VERSION" /tmp/install.sh && rm /tmp/install.sh

# RKE2 images
RUN mkdir -p ${IMAGES_DIR}
RUN curl -sfL "${RKE2_IMAGES_URL}/rke2-images.linux-amd64.txt" -o "${IMAGES_DIR}/rke2-images.linux-amd64.txt"
RUN curl -sfL "${RKE2_IMAGES_URL}/rke2-images.linux-amd64.tar.zst" -o "${IMAGES_DIR}/rke2-images.linux-amd64.tar.zst"
RUN curl -sfL "${RKE2_IMAGES_URL}/rke2-images.linux-amd64.txt" -o "${IMAGES_DIR}/rke2-images.linux-amd64.txt"
RUN curl -sfL "${RKE2_IMAGES_URL}/rke2-images-multus.linux-amd64.tar.zst" -o "${IMAGES_DIR}/rke2-images-multus.linux-amd64.tar.zst"
RUN curl -sfL "${RKE2_IMAGES_URL}/rke2-images-multus.linux-amd64.txt" -o "${IMAGES_DIR}/rke2-images-multus.linux-amd64.txt"

# rancherd dev version
RUN curl -L https://github.com/bk201/rancherd/releases/download/dev-20210707/rancherd > /usr/bin/rancherd && \
    chmod +x /usr/bin/rancherd

ENV CATTLE_SYSTEM_AGENT_VERSION v0.0.1-alpha34
RUN mkdir -p /var/lib/harvester/binaries/ && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/rancher-system-agent-amd64 -o /var/lib/harvester/binaries/rancher-system-agent && \
    chmod +x /var/lib/harvester/binaries/rancher-system-agent

# Harvester images
COPY images /var/lib/harvester/images

COPY files/ /
