FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

COPY audit-syslog-bridge/go.mod audit-syslog-bridge/go.sum* ./audit-syslog-bridge/
WORKDIR /src/audit-syslog-bridge
RUN go mod download

WORKDIR /src
COPY audit-syslog-bridge/ ./audit-syslog-bridge/
WORKDIR /src/audit-syslog-bridge

ARG VERSION=dev
# Cross-compile on the build host ($BUILDPLATFORM above) rather than under
# QEMU emulation: multi-arch release builds of the arm64 image were slow
# enough to hit the release job timeout.
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/audit-syslog-bridge .

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/audit-syslog-bridge /audit-syslog-bridge
USER 65532:65532
EXPOSE 8514
ENTRYPOINT ["/audit-syslog-bridge"]
