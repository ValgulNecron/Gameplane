FROM golang:1.25-alpine AS build
WORKDIR /src

COPY agent/go.mod agent/go.sum* ./agent/
WORKDIR /src/agent
RUN go mod download

WORKDIR /src
COPY agent/ ./agent/
WORKDIR /src/agent

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/agent ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/agent /agent
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/agent"]
