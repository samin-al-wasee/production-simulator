# ForgeLab development container.
# Base image provides Go, Git, curl, and common build tooling.
FROM mcr.microsoft.com/devcontainers/go:1

ARG TOOLS="\
  make \
  jq \
  shellcheck"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ${TOOLS} \
    && rm -rf /var/lib/apt/lists/*