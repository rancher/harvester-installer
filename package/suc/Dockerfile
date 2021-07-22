# FROM 8e8508cbe3a7
FROM ibuildthecloud/suc:latest

COPY install.sh /opt/rancher-system-agent-suc/
RUN wget https://github.com/rancher/system-agent/releases/download/v0.0.1-alpha34/rancher-system-agent-amd64 -O /opt/rancher-system-agent-suc/rancher-system-agent
RUN chmod +x /opt/rancher-system-agent-suc/rancher-system-agent
