# Probe image for Maven endpoint reachability. Built with BuildKit so the Azure
# DevOps token can be mounted as a secret rather than baked into a layer.
FROM mcr.microsoft.com/openjdk/jdk:17-azurelinux

# curl + CA certs for the TLS probes (tdnf is the Azure Linux package manager).
RUN tdnf install -y ca-certificates curl >/dev/null 2>&1 || true

COPY maven-probe.sh /maven-probe.sh

# Cache-buster so the probe RUN is never served from the layer cache.
ARG CACHE_BUST=0
RUN --mount=type=secret,id=aztoken \
    CACHE_BUST="${CACHE_BUST}" bash /maven-probe.sh
