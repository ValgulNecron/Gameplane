# Gameplane game server image for Nuclear Option (dedicated server).
#
# This image builds on the shared SteamCMD base (images/common/steamcmd/).
# The game server is installed at container startup via steam-install.sh,
# so the very first start downloads the game (~900 MB installed).
#
# Usage (container networking — port 7779 is loopback-only and unreachable outside the pod):
#   docker build -t gameplane-nuclear-option:latest images/games/nuclear-option
#   docker run -it \
#     -v game-data:/data \
#     -p 7778:7778/udp \
#     gameplane-nuclear-option:latest
#
# Note: Port 7779 (remote commands) binds to 127.0.0.1 only and cannot be reached from
# outside the container's network namespace. In Kubernetes, the agent sidecar reaches it
# via the shared pod network namespace; port mapping is not applicable.

# Base image reference. Allows CI to inject the exact digest.
# CI passes: --build-arg STEAMCMD_BASE_IMAGE=ghcr.io/valgulnecron/gameplane/common-steamcmd@sha256:...
ARG STEAMCMD_BASE_IMAGE=gameplane-steamcmd-base:latest
FROM ${STEAMCMD_BASE_IMAGE}

# Copy the game-specific entrypoint.
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Game images must define their own ENTRYPOINT (the base does not).
ENTRYPOINT ["/entrypoint.sh"]

# OCI metadata.
ARG VERSION=dev
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.title="Gameplane Nuclear Option Server"
LABEL org.opencontainers.image.description="Nuclear Option dedicated server for Gameplane"
LABEL org.opencontainers.image.source="https://github.com/valgulnecron/gameplane"
