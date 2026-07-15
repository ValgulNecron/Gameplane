FROM golang:1.25-alpine AS build
WORKDIR /src

# netguard and gameaction are in-repo modules the API depends on via local
# replaces (../netguard, ../gameaction); copy them first so `go mod
# download` and the build resolve them.
COPY netguard/ ./netguard/
COPY gameaction/ ./gameaction/
COPY api/go.mod api/go.sum* ./api/
WORKDIR /src/api
RUN go mod download

WORKDIR /src
COPY api/ ./api/
WORKDIR /src/api

ARG VERSION=dev
# modernc sqlite is pure Go, no CGO required.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/api ./cmd

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/api /api
USER 65532:65532
EXPOSE 8000
VOLUME ["/data"]
ENTRYPOINT ["/api"]
