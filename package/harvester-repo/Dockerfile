FROM nginx:stable

ARG PRELOAD_DIR=/usr/share/rancher/preload

COPY charts /usr/share/nginx/html/charts

# FIXME(kiefer): this doesn't sustain. A better way is to ask Rancher to serve binary too.
ENV CATTLE_SYSTEM_AGENT_VERSION v0.0.1-alpha34
RUN mkdir -p /usr/share/nginx/html/system-agent && \
    cd /usr/share/nginx/html/system-agent && \
    curl -sfL https://github.com/rancher/system-agent/releases/download/${CATTLE_SYSTEM_AGENT_VERSION}/install.sh -o system-agent-install.sh

RUN sed -i "/#!/a CATTLE_AGENT_BINARY_LOCAL=\"true\"\nCATTLE_AGENT_BINARY_LOCAL_LOCATION=\"${PRELOAD_DIR}/binaries/rancher-system-agent\"" /usr/share/nginx/html/system-agent/system-agent-install.sh
