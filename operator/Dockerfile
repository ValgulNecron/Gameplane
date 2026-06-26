FROM golang:1.25-alpine AS build
WORKDIR /src

# netguard is an in-repo module the operator depends on via a local replace
# (../netguard); copy it first so `go mod download` and the build resolve it.
COPY netguard/ ./netguard/
COPY operator/go.mod operator/go.sum* ./operator/
WORKDIR /src/operator
RUN go mod download

WORKDIR /src
COPY operator/ ./operator/
WORKDIR /src/operator
# VERSION is injected into main.Version so the operator reports its real build
# version (it compares this against a module bundle's gameplaneMinVersion). Keep
# this consistent with api/Dockerfile and agent/Dockerfile, which do the same.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" -o /out/manager ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
