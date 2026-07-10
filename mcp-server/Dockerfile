FROM golang:1.25-alpine AS build
WORKDIR /src

COPY mcp-server/go.mod mcp-server/go.sum* ./mcp-server/
WORKDIR /src/mcp-server
RUN go mod download

WORKDIR /src
COPY mcp-server/ ./mcp-server/
WORKDIR /src/mcp-server

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/mcp-server .

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/mcp-server /mcp-server
USER 65532:65532
# No EXPOSE: this server speaks MCP (JSON-RPC 2.0) over stdio only, never a
# network port. Reach a running instance with `kubectl exec -i ... -- /mcp-server
# serve` — see README.md.
ENTRYPOINT ["/mcp-server"]
