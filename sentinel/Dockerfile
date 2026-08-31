FROM golang:1.27-alpine AS build
WORKDIR /src

# gameproto is an in-repo module the sentinel depends on via a local replace
# (../gameproto); copy it first so `go mod download` and the build resolve
# it. Mirrors agent/Dockerfile and api/Dockerfile for the same reason.
COPY gameproto/ ./gameproto/
COPY sentinel/go.mod sentinel/go.sum* ./sentinel/
WORKDIR /src/sentinel
RUN go mod download

WORKDIR /src
COPY sentinel/ ./sentinel/
WORKDIR /src/sentinel

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/sentinel .

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/sentinel /sentinel
USER 65532:65532
EXPOSE 25565/tcp 25565/udp 19133/udp 27015/udp
ENTRYPOINT ["/sentinel"]
