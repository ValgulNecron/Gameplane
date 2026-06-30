FROM golang:1.25-alpine AS build
WORKDIR /src

COPY audit-syslog-bridge/go.mod audit-syslog-bridge/go.sum* ./audit-syslog-bridge/
WORKDIR /src/audit-syslog-bridge
RUN go mod download

WORKDIR /src
COPY audit-syslog-bridge/ ./audit-syslog-bridge/
WORKDIR /src/audit-syslog-bridge

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/audit-syslog-bridge .

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/audit-syslog-bridge /audit-syslog-bridge
USER 65532:65532
EXPOSE 8514
ENTRYPOINT ["/audit-syslog-bridge"]
